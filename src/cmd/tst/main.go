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
    "google.golang.org/api/mybusinessaccountmanagement/v1"
    "os"
)

func main() {
    lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    log := logger.NewLogger()
    stage := os.Getenv(util.StageEnvKey)
    log.Infof("Received request in %s: %s", stage, jsonUtil.AnyToJson(request))

    // DEBUG list files
    // err := filepath.Walk("/opt",
    //     func(path string, info os.FileInfo, err error) error {
    //         if err != nil {
    //             return err
    //         }
    //         fmt.Println(path, info.Size())
    //         return nil
    //     })
    // if err != nil {
    //     log.Error(err)
    // }

    const srcUserId = "U1de8edbae28c05ac8c7435bbd19485cb"     // 今遇良研
    const sendingUserId = "Ucc29292b212e271132cee980c58e94eb" // IL alpha

    // --------------------
    // initialize resources
    // --------------------
    // DDB
    mySession := session.Must(session.NewSession())
    userDao := ddbDao.NewUserDao(dynamodb.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1")), log)

    // --------------------
    // validate user exists
    // --------------------
    isUserExist, _, err := userDao.IsUserExist(srcUserId)
    if err != nil {
        log.Error("Error checking if user exists: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error checking if user exists"}`, StatusCode: 500}, nil
    }
    if !isUserExist {
        log.Error("User does not exist: ", srcUserId)
        return events.LambdaFunctionURLResponse{Body: `{"message": "User does not exist"}`, StatusCode: 400}, nil
    }

    log.Debugf("User %s exists, proceeding", srcUserId)

    // --------------------
    // Get metrics from Google
    // --------------------
    // TODO: how to get google user from user ID? Can populate manually, but how to get user to self-serve during onboarding?
    //
    // client, err := google.DefaultClient(context.TODO(),
    //     "https://www.googleapis.com/auth/devstorage.full_control")
    // if err != nil {
    //     log.Fatal(err)
    // }
    // client.Get("...")

    // businessprofileperformanceService, err := businessprofileperformance.NewService(context.Background())

    mybusinessaccountmanagementService, err := mybusinessaccountmanagement.NewService(context.Background())
    if err != nil {
        log.Error("Error creating Google business account management service: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error creating Google business account management service"}`, StatusCode: 500}, nil
    }

    resp, err := mybusinessaccountmanagementService.Accounts.List().Do()
    if err != nil {
        log.Error("Error listing Google business accounts: ", err)

        return events.LambdaFunctionURLResponse{
            Body:       `{"message": "Error listing Google business accounts"}`,
            StatusCode: 500,
        }, nil
    }

    log.Info("Successfully listed Google business accounts: ", jsonUtil.AnyToJson(resp))

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

    return events.LambdaFunctionURLResponse{Body: `{"message": "OK"}`, StatusCode: 200}, nil
}

func removeGoogleTranslate(event *model.ZapierNewReviewEvent) {
    text := event.Review

    originalLine, translationFound := util.ExtractOriginalFromGoogleTranslate(text)
    if translationFound {
        event.Review = originalLine
    }
}
