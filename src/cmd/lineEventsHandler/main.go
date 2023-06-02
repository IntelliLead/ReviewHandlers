package main

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/IntelliLead/ReviewHandlers/tst/data/lineEventsHandlerTestEvents"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
    "os"
)

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    log := logger.NewLogger()

    log.Info("Received new request: ", util.AnyToJson(request))

    // Check if the request is a health check call
    isHealthCheckCall, err := handleHealthCheckCall(request, log)
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
            Body:       `{"message": "OK"}`,
        }, nil
    }

    // initialize resources
    // DDB
    mySession := session.Must(session.NewSession())
    dao := ddbDao.NewUserDao(dynamodb.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1")), log)

    // LINE
    line := lineUtil.NewLineUtil(log)

    // parse message to LINE events
    var lineEvents []*linebot.Event
    if os.Getenv("Stage") == enum.StageLocal.String() {
        // DEBUG: test event
        lineEvents = lineEventsHandlerTestEvents.TestFollowEvent
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

    // process LINE events
    for _, event := range lineEvents {
        log.Infof("Processing event: %s\n", util.AnyToJson(event))

        switch event.Type {
        case linebot.EventTypeMessage:
            // TODO: revise quick reply
            // DEBUG: test handle
            response, err := line.SendQuickReply(event.ReplyToken)
            if err != nil {
                log.Error("Error sending message:", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to send message: %s"}`, err),
                }, nil
            }
            log.Debug("Response:", response)

        case linebot.EventTypeFollow:
            log.Info("Received Follow event")
            log.Info("Follow event User ID", event.Source)

            // if not exists, create new user in DB
            user := model.NewUser(event.Source.UserID, event.Timestamp)
            err := dao.CreateUser(user)
            if err != nil {
                if userAlreadyExistErr, ok := err.(*exception.UserAlreadyExistException); ok {
                    log.Info("User already exists. No action taken on Follow event:", userAlreadyExistErr.Error())
                    // return 200 OK
                } else {
                    log.Error("Error creating user:", err)

                    return events.LambdaFunctionURLResponse{
                        StatusCode: 500,
                        Body:       fmt.Sprintf(`{"error": "Failed to create user: %s"}`, err),
                    }, nil
                }
            } else {
                log.Info("Created new user:", user)
            }

        default:
            log.Info("Unhandled event type: ", event.Type)
        }
    }

    // Return a 200 OK response
    return events.LambdaFunctionURLResponse{Body: `{"message": "OK"}`, StatusCode: 200}, nil
}

/*
The LINE Platform may send an HTTP POST request that doesn't include a webhook event to confirm communication. In this case, send a 200 status code.

Parameters:

	request - The request from the LINE Messaging webhook source

Returns:

	bool - true if the request is a health check call and was handled, false otherwise
*/
func handleHealthCheckCall(request events.LambdaFunctionURLRequest, log *zap.SugaredLogger) (bool, error) {
    var body map[string]interface{}
    err := json.Unmarshal([]byte(request.Body), &body)
    if err != nil {
        log.Error("Error parsing request body:", err)
        return false, err
    }

    events, ok := body["events"].([]interface{})
    if !ok || len(events) == 0 {
        log.Info("Request doesn't include any events. Likely a health check call.")
        return true, nil
    }

    return false, nil
}

func main() {
    lambda.Start(handleRequest)
}
