package main

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "github.com/IntelliLead/CoreCommonUtil/enum"
    "github.com/IntelliLead/CoreCommonUtil/jsonUtil"
    "github.com/IntelliLead/CoreCommonUtil/logger"
    "github.com/IntelliLead/CoreCommonUtil/middleware"
    "github.com/IntelliLead/CoreCommonUtil/stringUtil"
    "github.com/IntelliLead/CoreDataAccess/ddbDao"
    "github.com/IntelliLead/CoreDataAccess/exception"
    "github.com/IntelliLead/CoreDataAccess/model"
    "github.com/IntelliLead/CoreDataAccess/model/type/bid"
    "github.com/IntelliLead/CoreDataAccess/model/type/rid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineEventProcessor"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/google/uuid"
    "os"
    "strings"
)

func main() {
    lambda.Start(middleware.MetricMiddleware(enum2.HandlerNameNewReviewEventHandler.String(), handleRequest))
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
    err = event.Validate()
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

    /*
       1. Extract business ID from event and get business from DB
       2. If business is not found, user is not authed. Use userId as partition key to create new review
    */
    var business model.Business
    // VendorReviewId is in the format of "accounts/BUSINESS_ACCOUNT_ID/locations/BUSINESS_ID/reviews/BUSINESS_REVIEW_ID"
    businessId, err := bid.NewBusinessId(strings.Split(event.VendorReviewId, "/")[3])
    if err != nil {
        log.Errorf("Error parsing business ID from vendorReviewId '%s': %v", event.VendorReviewId, err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error parsing business ID from vendorReviewId"}`, StatusCode: 500}, nil
    }
    businessPtr, err := businessDao.GetBusiness(businessId)
    if err != nil {
        log.Errorf("Error getting business '%s': %v", businessId, err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error getting business"}`, StatusCode: 500}, nil
    }

    var review model.Review
    if businessPtr == nil {
        log.Infof("Business '%s' does not exist.", businessId)

        if stringUtil.IsEmptyStringPtr(event.UserId) {
            log.Errorf("No business ID in event and no user ID in event. Unable to create new review")
            return events.LambdaFunctionURLResponse{Body: `{"message": "No business ID in event and no user ID in event"}`, StatusCode: 400}, nil
        }
        userId := *event.UserId

        log.Infof("User %s has not completed oauth. Assigning its userId as review partition key", userId)

        reviewId, err := reviewDao.GetNextReviewIDByUserId(userId)
        if err != nil {
            log.Errorf("Error getting next review id for user %s: %v", userId, err)
            return events.LambdaFunctionURLResponse{Body: `{"message": "Error getting next review id"}`, StatusCode: 500}, nil
        }

        review, err = model.NewReview(userId, reviewId, event)
        if err != nil {
            log.Error("Validation error occurred during mapping to Review object: ", err)
            return events.LambdaFunctionURLResponse{Body: `{"message": "Validation error during mapping to Review object"}`, StatusCode: 400}, nil
        }
    } else {
        business = *businessPtr

        var reviewId rid.ReviewId
        if !stringUtil.IsEmptyStringPtr(event.UserId) {
            reviewId, err = reviewDao.GetNextReviewID(business.BusinessId.String(), *event.UserId)
        } else {
            reviewId, err = reviewDao.GetNextReviewID(business.BusinessId.String(), "stub_user_id")
        }
        if err != nil {
            log.Errorf("Error getting next review id for business %s: %v", business.BusinessId, err)
            return events.LambdaFunctionURLResponse{Body: `{"message": "Error getting next review id"}`, StatusCode: 500}, nil
        }

        review, err = model.NewReview(business.BusinessId.String(), reviewId, event)
    }

    // local testing: generate a new vendor review ID to prevent duplication
    if stage == enum.StageLocal.String() {
        review.VendorReviewId = uuid.New().String()
    }

    log.Debug("Storing new review object from request: ", jsonUtil.AnyToJson(review))

    // --------------------
    // store review
    // --------------------
    err = reviewDao.PutReview(review)
    if err != nil {
        log.Error("Error creating review: ", err)

        var reviewAlreadyExistException exception.ReviewAlreadyExistException
        switch {
        case errors.As(err, &reviewAlreadyExistException):
            return events.LambdaFunctionURLResponse{Body: `{"message": "Review already exists"}`, StatusCode: 400}, nil
        default:
            return events.LambdaFunctionURLResponse{Body: `{"message": "Error creating review"}`, StatusCode: 500}, nil
        }
    }

    // --------------------------------
    // forward to LINE by calling LINE messaging API
    // --------------------------------
    if businessPtr != nil {
        business = *businessPtr
        err = line.SendNewReview(review, business, userDao)
        if err != nil {
            log.Errorf("Error sending new review to users of business '%s': %s", business.BusinessId, err)
            return events.LambdaFunctionURLResponse{Body: `{"message": "Error sending new review to LINE users of business"}`, StatusCode: 500}, nil
        }
        log.Info("Successfully sent new review to all users belonging to business: ", business.BusinessId)
    } else {
        if stringUtil.IsEmptyStringPtr(event.UserId) {
            log.Errorf("No business ID in event and no user ID in event. Unable to create new review")
            return events.LambdaFunctionURLResponse{Body: `{"message": "No business ID in event and no user ID in event"}`, StatusCode: 400}, nil
        }
        userId := *event.UserId

        err = line.SendNewReviewToUser(review, userId)
        if err != nil {
            log.Errorf("Error sending new review to user '%s': %s", userId, err)
            return events.LambdaFunctionURLResponse{Body: `{"message": "Error sending new review to LINE user"}`, StatusCode: 500}, nil
        }
        log.Info("Successfully sent new review to user: ", userId)
    }

    // --------------------------------
    // auto reply
    // --------------------------------
    if businessPtr != nil {
        autoQuickReplyEnabled := business.AutoQuickReplyEnabled
        quickReplyMessagePtr := business.QuickReplyMessage

        if autoQuickReplyEnabled && stringUtil.IsEmptyStringPtr(quickReplyMessagePtr) {
            log.Errorf("AutoQuickReplyEnabled set to true but no quickReplyMessage")
            return events.LambdaFunctionURLResponse{
                Body: `{"message": "Error getting quick reply message"}`, StatusCode: 500}, nil
        }

        if autoQuickReplyEnabled && stringUtil.IsEmptyStringPtr(review.Review) && review.NumberRating == 5 {
            quickReplyMessage := *quickReplyMessagePtr
            err = lineEventProcessor.ReplyReview(util.AutoReplyUserId, quickReplyMessage, review, reviewDao, log)
            if err != nil {
                log.Errorf("Error handling replying '%s' to review '%s' : %v", quickReplyMessage, review.ReviewId.String(), err)

                notifyUserErr := line.NotifyUsersReplyFailed(business.UserIds, review.ReviewerName, true)
                if notifyUserErr != nil {
                    log.Errorf("Error notifying users of business '%s' reply failed for review '%s': %v", businessId, review.ReviewId.String(), notifyUserErr)
                    return events.LambdaFunctionURLResponse{
                        StatusCode: 500,
                        Body:       fmt.Sprintf(`{"error": "Auto reply failed: %s. Failed to notify user of failure: %s"}`, err, notifyUserErr),
                    }, nil
                }

                log.Infof("Successfully notified users of business '%s' auto reply failed for review '%s'", businessId, review.ReviewId.String())

                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Reply failed: %s"}`, err),
                }, err
            }

            // --------------------
            // Notify review quick replied
            // --------------------
            err = line.NotifyReviewAutoReplied(review, quickReplyMessage, business, userDao)
            if err != nil {
                log.Errorf("Error sending review reply notification to all users of business '%s': %v", business.BusinessId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to send review reply notification to all users of business '%s': %v"}`, business.BusinessId, err),
                }, err
            }

            log.Infof("Successfully auto replied review for business '%s' for review '%s'", business.BusinessId, review.ReviewId.String())
        }
    }

    log.Info("Successfully processed new review event: ", jsonUtil.AnyToJson(review))

    return events.LambdaFunctionURLResponse{Body: `{"message": "OK"}`, StatusCode: 200}, nil
}

func removeGoogleTranslate(event *model.ZapierNewReviewEvent) {
    if event.Review == nil {
        return
    }
    text := *event.Review

    originalLine, translationFound := stringUtil.ExtractOriginalFromGoogleTranslate(text)
    if translationFound {
        event.Review = &originalLine
    }
}
