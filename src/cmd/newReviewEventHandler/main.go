package main

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/auth"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineEventProcessor"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/middleware"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/go-playground/validator/v10"
    "github.com/google/uuid"
    "os"
)

func main() {
    lambda.Start(middleware.MetricMiddleware(enum.HandlerNameNewReviewEventHandler.String(), handleRequest))
}

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    log := logger.NewLogger()
    stage := os.Getenv(util.StageEnvKey)
    log.Infof("Received request in %s: %s", stage, jsonUtil.AnyToJson(request))

    // --------------------
    // parse request body
    // --------------------
    var event model.ZapierNewReviewEvent
    err := json.Unmarshal([]byte(request.Body), &event)
    if err != nil {
        log.Error("Error parsing request body: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error parsing request body"}`, StatusCode: 400}, nil
    }
    err = validator.New().Struct(event)
    if err != nil {
        // Handle validation error
        log.Error("Validation error when parsing request body: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Validation error when parsing request body"}`, StatusCode: 400}, nil
    }

    // Google may translate Mandarin to English. Remove the translation
    removeGoogleTranslate(&event)

    // --------------------
    // initialize resources
    // --------------------
    // DDB
    cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-northeast-1"))
    if err != nil {
        log.Error("Error loading AWS config: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error loading AWS config"}`, StatusCode: 500}, nil
    }
    businessDao := ddbDao.NewBusinessDao(dynamodb.NewFromConfig(cfg), log)
    userDao := ddbDao.NewUserDao(dynamodb.NewFromConfig(cfg), log)
    reviewDao := ddbDao.NewReviewDao(dynamodb.NewFromConfig(cfg), log)

    // --------------------
    // map to Review object
    // --------------------
    hasUserCompletedOauth, user, business, err := auth.ValidateUserAuth(event.UserId, userDao, businessDao, log)
    if err != nil {
        log.Errorf("Error checking if user %s has completed oauth: %s", event.UserId, err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error checking if user has completed oauth"}`, StatusCode: 500}, nil
    }

    businessId := event.UserId
    if hasUserCompletedOauth {
        log.Infof("User %s has completed oauth. Assigning its businessId '%s' as review partition key", event.UserId, *user.ActiveBusinessId)
        businessId = *user.ActiveBusinessId
    } else {
        log.Infof("User %s has not completed oauth. Assigning its userId as review partition key", event.UserId)
    }

    reviewPtr, err := model.NewReview(businessId, event)
    if err != nil {
        log.Error("Validation error occurred during mapping to Review object: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Validation error during mapping to Review object"}`, StatusCode: 400}, nil
    }
    review := *reviewPtr
    // local testing: generate a new vendor review ID to prevent duplication
    if stage == enum.StageLocal.String() {
        review.VendorReviewId = uuid.New().String()
    }

    log.Debug("Review object from request: ", jsonUtil.AnyToJson(review))

    // --------------------
    // store review
    // --------------------
    // TODO: Remove userId field after [INT-97] is done
    nextReviewId, err := reviewDao.GetNextReviewID(review.BusinessId, user.UserId)
    // nextReviewId, err := reviewDao.GetNextReviewID(review.BusinessId)
    if err != nil {
        log.Errorf("Error getting next review id for BusinessId %s: %v", review.BusinessId, err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error getting next review id"}`, StatusCode: 500}, nil
    }

    review.ReviewId = &nextReviewId

    log.Infof("Assigned review id %s to new review", nextReviewId.String())

    err = reviewDao.CreateReview(review)
    if err != nil {
        log.Error("Error creating review: ", err)

        var reviewAlreadyExistException exception.ReviewAlreadyExistException
        var vendorReviewIdAlreadyExistException exception.VendorReviewIdAlreadyExistException
        switch {
        case errors.As(err, &reviewAlreadyExistException):
            return events.LambdaFunctionURLResponse{Body: `{"message": "Review already exists"}`, StatusCode: 400}, nil
        case errors.As(err, &vendorReviewIdAlreadyExistException):
            log.Error("Create review failed because vendorReviewId unique record exists but not the review object: ", review)
            return events.LambdaFunctionURLResponse{Body: `{"message": "Database conflict"}`, StatusCode: 500}, nil
        default:
            return events.LambdaFunctionURLResponse{Body: `{"message": "Error creating review"}`, StatusCode: 500}, nil
        }
    }

    // --------------------------------
    // forward to LINE by calling LINE messaging API
    // --------------------------------
    line := lineUtil.NewLine(log)

    // TODO: [INT-97] Remove passing in user and change pass in business object when all users are backfilled with active business ID
    err = line.SendNewReview(review, business, *user)
    if err != nil {
        log.Errorf("Error sending new review to LINE user %s: %s", user.UserId, jsonUtil.AnyToJson(err))
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error sending new review to LINE"}`, StatusCode: 500}, nil
    }
    log.Info("Successfully sent new review to all users belonging to business: ", business.BusinessId)

    // TODO: [INT-97] Remove this check when all users are backfilled with active business ID
    if user.ActiveBusinessId == nil {
        log.Errorf("User %s has no active business. Cannot perform auto reply", user.UserId)
        return events.LambdaFunctionURLResponse{Body: `{"message": "User has no active business. Cannot perform auto reply"}`, StatusCode: 200}, nil
    }

    autoQuickReplyEnabled := business.AutoQuickReplyEnabled
    quickReplyMessage := business.QuickReplyMessage

    if autoQuickReplyEnabled && util.IsEmptyStringPtr(review.Review) && review.NumberRating == 5 {
        if quickReplyMessage == nil {
            log.Error("User has autoQuickReplyEnabled but no quickReplyMessage: %s", user.UserId)
            return events.LambdaFunctionURLResponse{
                Body: `{"message": "Error getting quick reply message"}`, StatusCode: 500}, nil
        }

        err = lineEventProcessor.ReplyReview(business.BusinessId, user.UserId, *quickReplyMessage, review, reviewDao, log)
        if err != nil {
            log.Errorf("Error handling replying '%s' to review '%s' for user '%s' of business '%s': %v", *quickReplyMessage, review.ReviewId.String(), user.UserId, business.BusinessId, err)

            _, notifyUserErr := line.ReplyUserReplyFailed(user.UserId, review.ReviewerName, false)
            if notifyUserErr != nil {
                log.Errorf("Error notifying user '%s' reply failed for review '%s': %v", user.UserId, review.ReviewId.String(), notifyUserErr)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to notify reply failure for user '%s' : %v. Reply Failure reason: %v"}`, user.UserId, notifyUserErr, err),
                }, notifyUserErr
            }

            log.Infof("Successfully notified user '%s' of business '%s' reply to review '%s' failed: %v",
                user.UserId, business.BusinessId, review.ReviewId.String(), err)

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Reply failed: %s"}`, err),
            }, err
        }

        // send LINE message
        // --------------------
        err = line.NotifyReviewReplied(business.UserIds, nil, nil, review, *quickReplyMessage, "自動回覆", true)
        if err != nil {
            log.Errorf("Error sending review reply notification to all users of business '%s': %v", businessId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to send review reply notification to all users of business '%s': %v"}`, businessId, err),
            }, err
        }

        log.Infof("Successfully auto replied review for business '%s' for review '%s'", businessId, review.ReviewId.String())
    }

    log.Info("Successfully processed new review event: ", jsonUtil.AnyToJson(review))

    return events.LambdaFunctionURLResponse{Body: `{"message": "OK"}`, StatusCode: 200}, nil
}

func removeGoogleTranslate(event *model.ZapierNewReviewEvent) {
    if event.Review == nil {
        return
    }
    text := *event.Review

    originalLine, translationFound := util.ExtractOriginalFromGoogleTranslate(text)
    if translationFound {
        event.Review = &originalLine
    }
}
