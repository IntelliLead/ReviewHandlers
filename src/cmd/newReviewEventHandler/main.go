package main

import (
    "context"
    "encoding/json"
    "errors"
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

    reviewPtr, err := model.NewReview(event)
    if err != nil {
        log.Error("Validation error during mapping to Review object: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Validation error during mapping to Review object"}`, StatusCode: 400}, nil
    }
    review := *reviewPtr
    // local testing: generate a new vendor review ID to prevent duplication
    if stage == enum.StageLocal.String() {
        review.VendorReviewId = uuid.New().String()
    }

    log.Debug("Review object from request: ", jsonUtil.AnyToJson(review))

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
    // store review
    // --------------------
    nextReviewId, err := reviewDao.GetNextReviewID(review.BusinessId)
    if err != nil {
        log.Errorf("Error getting next review id for BusinessId %s: %v", review.BusinessId, err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error getting next review id"}`, StatusCode: 500}, nil
    }

    review.ReviewId = &nextReviewId

    log.Debugf("Assigned review id %s to new review", nextReviewId.String())

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

    // TODO: [INT-94] Send to all users in the business instead of relying on the review-associated user (legacy)
    // --------------------
    // validate user exists
    // --------------------
    user, err := userDao.GetUser(review.UserId)
    if err != nil {
        log.Error("Error getting user: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error getting user"}`, StatusCode: 500}, nil
    }
    if user == nil {
        log.Error("User does not exist: ", review.UserId)
        return events.LambdaFunctionURLResponse{Body: `{"message": "User does not exist"}`, StatusCode: 400}, nil
    }

    log.Debugf("User %s exists, proceeding", review.UserId)

    // --------------------------------
    // forward to LINE by calling LINE messaging API
    // --------------------------------
    line := lineUtil.NewLine(log)

    err = line.SendNewReview(review, *user)
    if err != nil {
        log.Errorf("Error sending new review to LINE user %s: %s", review.BusinessId, jsonUtil.AnyToJson(err))
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error sending new review to LINE"}`, StatusCode: 500}, nil
    }

    log.Debugf("Successfully sent new review to LINE user: '%s'", review.BusinessId)

    // TODO: [INT-91] Remove backfill logic once all users have been backfilled
    var autoQuickReplyEnabled bool
    var quickReplyMessage *string
    if user.ActiveBusinessId != nil {
        business, err := businessDao.GetBusiness(*user.ActiveBusinessId)
        if err != nil {
            log.Errorf("Error getting business %s: %s", *user.ActiveBusinessId, jsonUtil.AnyToJson(err))
            return events.LambdaFunctionURLResponse{
                Body: `{"message": "Error getting business"}`, StatusCode: 500}, err
        }
        autoQuickReplyEnabled = business.AutoQuickReplyEnabled
        quickReplyMessage = business.QuickReplyMessage
    } else {
        if user.AutoQuickReplyEnabled == nil {
            log.Warn("User is not backfilled but has no autoQuickReplyEnabled flag: %s", user.UserId)
            autoQuickReplyEnabled = false
        } else {
            autoQuickReplyEnabled = *user.AutoQuickReplyEnabled
            quickReplyMessage = user.QuickReplyMessage
        }
    }

    if autoQuickReplyEnabled && util.IsEmptyStringPtr(review.Review) && review.NumberRating == 5 {
        if quickReplyMessage == nil {
            log.Error("User has autoQuickReplyEnabled but no quickReplyMessage: %s", user.UserId)
            return events.LambdaFunctionURLResponse{
                Body: `{"message": "Error getting quick reply message"}`, StatusCode: 501}, nil
        }

        lambdaReturn, err := lineEventProcessor.ReplyReview(
            user.UserId, nil, *quickReplyMessage, review, reviewDao, line, log, true)
        if err != nil {
            return lambdaReturn, err
        }

        log.Infof("Successfully auto replied review for user ID '%s' for review ID '%s'",
            user.UserId, review.ReviewId.String())

        return lambdaReturn, nil
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
