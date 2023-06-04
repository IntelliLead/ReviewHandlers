package main

import (
    "context"
    "encoding/json"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/go-playground/validator/v10"
)

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    log := logger.NewLogger()
    log.Info("Received request: ", util.AnyToJson(request))
    log.Debug("Received request header: ", request.Headers)
    log.Debug("Received request body: ", request.Body)

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

    reviewPtr, err := model.NewReview(event)
    if err != nil {
        log.Error("Validation error during mapping to Review object: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Validation error during mapping to Review object"}`, StatusCode: 400}, nil
    }
    review := *reviewPtr
    log.Debug("Review object from request: ", util.AnyToJson(review))

    // --------------------
    // initialize resources
    // --------------------
    // DDB
    mySession := session.Must(session.NewSession())
    userDao := ddbDao.NewUserDao(dynamodb.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1")), log)
    reviewDao := ddbDao.NewReviewDao(dynamodb.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1")), log)

    // --------------------
    // validate user exists
    // --------------------
    isUserExist, err := userDao.IsUserExist(review.UserId)
    if err != nil {
        log.Error("Error checking if user exists: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error checking if user exists"}`, StatusCode: 500}, nil
    }
    if !isUserExist {
        log.Error("User does not exist: ", review.UserId)
        return events.LambdaFunctionURLResponse{Body: `{"message": "User does not exist"}`, StatusCode: 400}, nil
    }

    log.Debugf("User %s exists, proceeding", review.UserId)

    // --------------------
    // store review
    // --------------------
    nextReviewId, err := reviewDao.GetNextReviewID(review.UserId)
    if err != nil {
        log.Errorf("Error getting next review id for userId %s: %v", review.UserId, err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error getting next review id"}`, StatusCode: 500}, nil
    }

    review.ReviewId = &nextReviewId

    log.Debugf("Assigned review id %s to new review", nextReviewId.String())

    err = reviewDao.CreateReview(review)
    if err != nil {
        log.Error("Error creating review: ", err)

        switch err.(type) {
        case exception.ReviewAlreadyExistException:
            return events.LambdaFunctionURLResponse{Body: `{"message": "Review already exists"}`, StatusCode: 400}, nil
        case exception.VendorReviewIdAlreadyExistException:
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
    
    err = line.SendNewReview(review)
    if err != nil {
        log.Errorf("Error sending new review to LINE user %s: %s", review.UserId, util.AnyToJson(err))
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error sending new review to LINE"}`, StatusCode: 500}, nil
    }

    // --------------------
    log.Info("Successfully processed new review event: ", review)

    return events.LambdaFunctionURLResponse{Body: `{"message": "OK"}`, StatusCode: 200}, nil
}

func main() {
    lambda.Start(handleRequest)
}
