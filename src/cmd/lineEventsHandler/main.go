package main

import (
    "context"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineEventProcessor"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineEventProcessor/messageEvent"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineEventProcessor/postbackEvent"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/middleware"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/slackUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/IntelliLead/ReviewHandlers/tst/data/lineEventsHandlerTestEvents"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "os"
)

func main() {
    lambda.Start(middleware.MetricMiddleware(enum.HandlerNameLineEventsHandler.String(), handleRequest))
}

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    log := logger.NewLogger()
    stageStr := os.Getenv(util.StageEnvKey)
    stage := enum.StringToStage(stageStr) // panic if invalid stage

    log.Infof("Received new request in %s: %s", stage.String(), jsonUtil.AnyToJson(request))

    // --------------------
    // Check if the request is a health check call
    // --------------------
    isHealthCheckCall, err := lineEventProcessor.ProcessHealthCheckCall(request, log)
    if err != nil {
        log.Error("Error handling health check call:", err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 400,
            Body:       fmt.Sprintf(`{"error": "Failed to handle health check call. Malformat request? : %s"}`, err),
        }, nil
    }
    if isHealthCheckCall {
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Headers:    map[string]string{"Content-Type": "application/json"},
            Body:       `{"mesage": "OK"}`,
        }, nil
    }

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
    // LINE
    line := lineUtil.NewLine(log)

    // parse message to LINE events
    var lineEvents []*linebot.Event

    // This is useful for local development, where we can't/won't generate a new request with valid signature.
    // LINE events signature becomes invalid after a while (sometimes days). In this case, instead of generating a new request, we can opt to bypass event parser (signature check) and craft our own parsed line events.
    if stage == enum.StageLocal {
        log.Debug("Running in local environment. Skipping LINE event parser")
        lineEvents = lineEventsHandlerTestEvents.TestRichMenuAiReplySettingsEvent
    } else {
        err = nil
        lineEvents, err = line.ParseRequest(&request)
        if err != nil {
            // Log and return an error response
            log.Error("Error parsing Line Event request:", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 400,
                Body:       fmt.Sprintf(`{"error": "Failed to parse request: %s"}`, err),
            }, nil
        }
    }
    log.Infof("Received %d LINE events: ", len(lineEvents))

    // --------------------
    // process LINE events
    // --------------------
    for _, event := range lineEvents {
        log.Infof("Processing event: %s\n", jsonUtil.AnyToJson(event))

        if lineUtil.IsEventFromUser(event) == false {
            log.Info("Postback event is not from user. No action taken", jsonUtil.AnyToJson(event))
            return events.LambdaFunctionURLResponse{
                StatusCode: 200,
                Body:       `{"message": "Postback event is not from user. No action taken."}`,
            }, nil
        }
        userId := event.Source.UserID

        switch event.Type {
        case linebot.EventTypeMessage:
            log.Info("Received Message event")
            return messageEvent.ProcessMessageEvent(event, userId, businessDao, userDao, reviewDao, line, log)

        case linebot.EventTypeFollow:
            log.Info("Received Follow event")
            slack := slackUtil.NewSlack(log, stage)
            return lineEventProcessor.ProcessFollowEvent(event, userDao, slack, line, log)

        case linebot.EventTypePostback:
            log.Info("Received Postback event")
            return postbackEvent.ProcessPostbackEvent(event, userId, businessDao, userDao, reviewDao, line, log)

        default:
            log.Info("Unhandled event type: ", event.Type)
            if event.Type == linebot.EventTypePostback {
                log.Info("Postback data: ", event.Postback.Data)
            }
        }
    }

    // Return a 200 OK response
    return events.LambdaFunctionURLResponse{Body: `{"message": "OK"}`, StatusCode: 200}, nil
}
