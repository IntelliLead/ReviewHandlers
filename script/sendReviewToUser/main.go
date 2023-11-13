package main

import (
    "context"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/rid"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func main() {
    lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    log := logger.NewLogger()

    // --------------------
    // script parameters
    // --------------------
    businessId, err := bid.NewBusinessId("accounts/109717233744421630062/locations/3442155390184691720")
    if err != nil {
        log.Error("Error parsing business id: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error parsing business id"}`, StatusCode: 500}, nil
    }
    userId := "Ud5ff72b21621e6873262c463c04187c3"
    reviewIdStrs := []string{"q", "r", "s", "t", "u"}

    reviewIds := make([]rid.ReviewId, len(reviewIdStrs))
    for i, reviewIdStr := range reviewIdStrs {
        reviewId, err := rid.NewReviewId(reviewIdStr)
        if err != nil {
            log.Error("Error parsing review id: ", err)
            return events.LambdaFunctionURLResponse{Body: `{"message": "Error parsing review id"}`, StatusCode: 500}, nil
        }
        reviewIds[i] = reviewId
    }

    // --------------------
    // initialize resources
    // --------------------
    cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-northeast-1"))
    if err != nil {
        log.Error("Error loading AWS config: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error loading AWS config"}`, StatusCode: 500}, nil
    }
    reviewDao := ddbDao.NewReviewDao(dynamodb.NewFromConfig(cfg), log)
    userDao := ddbDao.NewUserDao(dynamodb.NewFromConfig(cfg), log)
    businessDao := ddbDao.NewBusinessDao(dynamodb.NewFromConfig(cfg), log)

    // LINE
    line := lineUtil.NewLine(log)

    // --------------------
    // retrieve business and send review
    // --------------------
    businessPtr, err := businessDao.GetBusiness(businessId)
    if err != nil {
        log.Errorf("Error getting business: %s", err)
        return events.LambdaFunctionURLResponse{}, err
    }
    if businessPtr == nil {
        log.Errorf("Business not found")
        return events.LambdaFunctionURLResponse{}, err
    }
    business := *businessPtr

    for _, reviewId := range reviewIds {
        review, err := reviewDao.GetReview(userId, reviewId)
        if err != nil {
            log.Errorf("Error getting review: %s", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 400,
                Body:       fmt.Sprintf(`{"error": "Failed to get review: %s"}`, err),
            }, err
        }
        if review == nil {
            log.Errorf("Review not found: %s", reviewId)
            return events.LambdaFunctionURLResponse{
                StatusCode: 400,
                Body:       fmt.Sprintf(`{"error": "Review not found: %s"}`, reviewId),
            }, err
        }

        err = line.SendNewReview(*review, business, userDao)
        if err != nil {
            log.Errorf("Error sending new review to user: %s", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 400,
                Body:       fmt.Sprintf(`{"error": "Failed to send new review to user: %s"}`, err),
            }, err
        }

    }

    // Return a 200 OK response
    return events.LambdaFunctionURLResponse{Body: `{"message": "OK"}`, StatusCode: 200}, nil
}
