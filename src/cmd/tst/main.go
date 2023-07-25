package main

import (
    "context"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "os"
)

func main() {
    lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    log := logger.NewLogger()
    stage := os.Getenv(util.StageEnvKey)
    log.Infof("Received request in %s: %s", stage, jsonUtil.AnyToJson(request))

    const userId = "U1de8edbae28c05ac8c7435bbd19485cb" // 今遇良研

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
    isUserExist, user, err := userDao.IsUserExist(userId)
    if err != nil {
        log.Error("Error checking if user exists: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error checking if user exists"}`, StatusCode: 500}, nil
    }
    if !isUserExist {
        log.Error("User does not exist: ", userId)
        return events.LambdaFunctionURLResponse{Body: `{"message": "User does not exist"}`, StatusCode: 400}, nil
    }

    log.Debugf("User %s exists, proceeding", userId)

    // --------------------
    // Get metrics from Google
    // --------------------
    // TODO: how to get google user from user ID? Can populate manually, but how to get user to self-serve during onboarding?

    // --------------------------------
    // forward to LINE by calling LINE messaging API
    // --------------------------------

    // line := lineUtil.NewLine(log)
    //
    // err = line.SendNewReview(review, user)
    // if err != nil {
    //     log.Errorf("Error sending new review to LINE user %s: %s", review.UserId, jsonUtil.AnyToJson(err))
    //     return events.LambdaFunctionURLResponse{Body: `{"message": "Error sending new review to LINE"}`, StatusCode: 500}, nil
    // }
    //
    // log.Debugf("Successfully sent new review to LINE user: '%s'", review.UserId)
    //
    // // --------------------
    // log.Info("Successfully processed new review event: ", jsonUtil.AnyToJson(review))
    //
    // return events.LambdaFunctionURLResponse{Body: `{"message": "OK"}`, StatusCode: 200}, nil
}

func removeGoogleTranslate(event *model.ZapierNewReviewEvent) {
    text := event.Review

    originalLine, translationFound := util.ExtractOriginalFromGoogleTranslate(text)
    if translationFound {
        event.Review = originalLine
    }
}
