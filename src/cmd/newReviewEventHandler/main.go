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
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/rid"
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
    lambda.Start(middleware.MetricMiddleware(enum.HandlerNameNewReviewEventHandler, handleRequest))
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
    err = validator.New(validator.WithRequiredStructEnabled()).Struct(event)
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
    line := lineUtil.NewLine(log)
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
    // validate user auth and get user/business
    // --------------------
    userId := event.UserId
    hasUserCompletedOauth, user, err := auth.ValidateUserAuth(userId, userDao, line, enum.HandlerNameNewReviewEventHandler, log)
    if err != nil {
        log.Errorf("Error checking if user %s has completed oauth: %s", userId, err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error checking if user has completed oauth"}`, StatusCode: 500}, nil
    }

    /*
       1. if there's businessID in event: use businessID as partition key to create review, and send to all users associated with the business
       2. if there's no business ID in event, and user is authed: the user should only have 1 associated business ID. Use it as in 1.
       3. if there's no business ID in event, and user is not authed: use userID as the partition key to create review, and only send to the user
    */
    var businessId bid.BusinessId
    hasBusiness := true // flag to indicate 3rd option
    if event.BusinessId != nil {
        businessId = *event.BusinessId
        businessIdsStr := make([]string, len(user.BusinessIds))
        for i, v := range user.BusinessIds {
            businessIdsStr[i] = string(v)
        }

        if len(user.BusinessIds) != 0 && !util.StringInSlice(businessId.String(), businessIdsStr) {
            log.Errorf("User '%s' is not associated with business '%s'", userId, businessId)
            return events.LambdaFunctionURLResponse{Body: `{"message": "User is not associated with business"}`, StatusCode: 500}, nil
        }
    }

    if hasUserCompletedOauth && event.BusinessId == nil {
        switch len(user.BusinessIds) {
        case 0:
            log.Errorf("User '%s' has completed OAUTH but has no business", userId)
            return events.LambdaFunctionURLResponse{Body: `{"message": "User has completed OAUTH but has no business"}`, StatusCode: 500}, nil
        case 1:
            businessId = user.BusinessIds[0]
        default:
            log.Error("User '%s' has completed OAUTH but has multiple businesses. Cannot determine the businessID to associate with this review.", userId)
            return events.LambdaFunctionURLResponse{Body: `{"message": "User has completed OAUTH but has multiple businesses. Cannot determine the businessID to associate with this review"}`, StatusCode: 500}, nil
        }
    } else {
        log.Infof("User %s has not completed oauth. Assigning its userId as review partition key", userId)
        hasBusiness = false
    }

    var business model.Business
    if hasBusiness {
        businessPtr, err := businessDao.GetBusiness(businessId)
        if err != nil {
            log.Errorf("Error getting business '%s' that should belong to user '%s': %v", businessId, userId, err)
            return events.LambdaFunctionURLResponse{Body: `{"message": "Error getting business"}`, StatusCode: 500}, nil
        }
        if businessPtr == nil {
            log.Errorf("Business '%s' does not exist", businessId)
            return events.LambdaFunctionURLResponse{Body: `{"message": "Business does not exist"}`, StatusCode: 500}, nil
        }
        business = *businessPtr
    }

    // --------------------
    // map to Review object
    // --------------------
    var reviewId rid.ReviewId
    // TODO: Remove userId field after [INT-97] is done
    // nextReviewId, err := reviewDao.GetNextReviewID(review.BusinessId)
    if hasBusiness {
        reviewId, err = reviewDao.GetNextReviewID(businessId.String(), userId)
        if err != nil {
            log.Errorf("Error getting next review id for business %s: %v", businessId, err)
            return events.LambdaFunctionURLResponse{Body: `{"message": "Error getting next review id"}`, StatusCode: 500}, nil
        }
    } else {
        reviewId, err = reviewDao.GetNextReviewID(userId, userId)
        if err != nil {
            log.Errorf("Error getting next review id for user %s: %v", userId, err)
            return events.LambdaFunctionURLResponse{Body: `{"message": "Error getting next review id"}`, StatusCode: 500}, nil
        }
    }

    log.Infof("Assigned review id %s to new review", reviewId.String())

    var review model.Review
    if hasBusiness {
        review, err = model.NewReview(businessId.String(), reviewId, event)
    } else {
        review, err = model.NewReview(userId, reviewId, event)
    }
    if err != nil {
        log.Error("Validation error occurred during mapping to Review object: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Validation error during mapping to Review object"}`, StatusCode: 400}, nil
    }

    // local testing: generate a new vendor review ID to prevent duplication
    if stage == enum.StageLocal.String() {
        review.VendorReviewId = uuid.New().String()
    }

    log.Debug("Review object from request: ", jsonUtil.AnyToJson(review))

    // --------------------
    // store review
    // --------------------
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
    if hasBusiness {
        err = line.SendNewReview(review, business)
        if err != nil {
            log.Errorf("Error sending new review to users of business '%s': %s", businessId, jsonUtil.AnyToJson(err))
            return events.LambdaFunctionURLResponse{Body: `{"message": "Error sending new review to LINE users of business"}`, StatusCode: 500}, nil
        }
        log.Info("Successfully sent new review to all users belonging to business: ", businessId)
    } else {
        err = line.SendNewReviewToUser(review, user)
        if err != nil {
            log.Errorf("Error sending new review to user '%s': %s", userId, jsonUtil.AnyToJson(err))
            return events.LambdaFunctionURLResponse{Body: `{"message": "Error sending new review to LINE user"}`, StatusCode: 500}, nil
        }
        log.Info("Successfully sent new review to user: ", userId)
    }

    // --------------------------------
    // auto reply
    // --------------------------------
    var autoQuickReplyEnabled bool
    var quickReplyMessagePtr *string
    if hasBusiness {
        autoQuickReplyEnabled = business.AutoQuickReplyEnabled
        quickReplyMessagePtr = business.QuickReplyMessage
    } else {
        if user.AutoQuickReplyEnabled == nil {
            log.Errorf("User '%s' has nil autoQuickReplyEnabled", user.UserId)
            autoQuickReplyEnabled = false
        } else {
            autoQuickReplyEnabled = *user.AutoQuickReplyEnabled
        }
        quickReplyMessagePtr = user.QuickReplyMessage
    }
    if autoQuickReplyEnabled && util.IsEmptyStringPtr(quickReplyMessagePtr) {
        log.Errorf("AutoQuickReplyEnabled set to true but no quickReplyMessage")
        return events.LambdaFunctionURLResponse{
            Body: `{"message": "Error getting quick reply message"}`, StatusCode: 500}, nil
    }

    if autoQuickReplyEnabled && util.IsEmptyStringPtr(review.Review) && review.NumberRating == 5 {
        quickReplyMessage := *quickReplyMessagePtr
        err = lineEventProcessor.ReplyReview(util.AutoReplyUserId, quickReplyMessage, review, reviewDao, log)
        if err != nil {
            log.Errorf("Error handling replying '%s' to review '%s' from user '%s': %v", quickReplyMessage, review.ReviewId.String(), userId, err)

            _, notifyUserErr := line.NotifyUserReplyFailed(userId, review.ReviewerName, true)
            if notifyUserErr != nil {
                log.Errorf("Error notifying user '%s' reply failed for review '%s': %v", userId, review.ReviewId.String(), notifyUserErr)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Auto reply failed: %s. Failed to notify user of failure: %s"}`, err, notifyUserErr),
                }, nil
            }

            log.Infof("Successfully notified user '%s' auto reply failed for review '%s'", userId, review.ReviewId.String())

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Reply failed: %s"}`, err),
            }, err
        }

        // --------------------
        // Notify review quick replied
        // --------------------
        if hasBusiness {
            err = line.NotifyReviewReplied(business.UserIds, nil, nil, review, quickReplyMessage, "自動回覆", true, &business.BusinessName)
        } else {
            err = line.NotifyReviewReplied([]string{userId}, nil, nil, review, quickReplyMessage, "自動回覆", true, nil)
        }
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
