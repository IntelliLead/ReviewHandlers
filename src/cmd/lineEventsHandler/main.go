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
	"github.com/IntelliLead/ReviewHandlers/src/pkg/util"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/line/line-bot-sdk-go/v7/linebot"
	"go.uber.org/zap"
	"log"
	"os"
	"time"
)

func newLineClient(logger *zap.SugaredLogger) *linebot.Client {
	// Create a new LINE Bot client
	channelSecret := "a6064245795375fee1fb9cc2e4711447"
	channelAccessToken := "0PWI55x6HFQ1WfHOBTddspgVTpTbFtFmy9ImN7NuYqScSz0mTFjYDqb9dA8TeRaUHNCrAWJ0x6yv4iJiMNrki4ZuYS4UhntFFtKma5tocBpgMcnD8+Kg0cTz3yoghq24QKmKp7R7OfoaTn4i/m7Y1AdB04t89/1O/w1cDnyilFU="
	lineClient, err := linebot.New(channelSecret, channelAccessToken)
	if err != nil {
		logger.Fatal("cannot create new Line Client", err)
	}

	return lineClient
}

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	logger := logger.InitLogger()

	logger.Info("Received new request: ", util.AnyToJson(request))

	// Check if the request is a health check call
	if handleHealthCheckCall(request) {
		return events.LambdaFunctionURLResponse{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `{"message": "OK"}`,
		}, nil
	}

	// initialize resources
	// DDB
	mySession := session.Must(session.NewSession())
	dao := ddbDao.NewDao(dynamodb.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1")), logger)

	// LINE
	line := lineUtil.NewLineUtil(newLineClient(logger), logger)

	// parse message to LINE events
	var lineEvents []*linebot.Event
	var err error = nil
	if os.Getenv("Stage") == model.StageLocal.String() {
		lineEvents =
			[]*linebot.Event{
				{
					Type:           linebot.EventTypeFollow,
					WebhookEventID: "01H1NCFZSJN1HAPFREM0193Y1Q",
					DeliveryContext: linebot.DeliveryContext{
						IsRedelivery: false,
					},
					// Timestamp: 1685418671895,
					// epoch ms to time.Time
					Timestamp: time.UnixMilli(1685418671895),
					Source: &linebot.EventSource{
						Type:   linebot.EventSourceTypeUser,
						UserID: "Ucc29292b212e271132cee980c58e94eb",
					},
					ReplyToken: "36ffd31138354b2dbe94d1a7759fb9ab",
					Mode:       linebot.EventModeActive,
				},
			}
	} else {
		lineEvents, err = line.ParseRequest(&request)
		if err != nil {
			// Log and return an error response
			logger.Error("Error parsing Line Event request:", err)
			return events.LambdaFunctionURLResponse{
				StatusCode: 400,
				Body:       fmt.Sprintf(`{"error": "Failed to parse request: %s"}`, err),
			}, nil
		}
	}
	logger.Infof("Received %d LINE events: ", len(lineEvents))

	// process LINE events
	for _, event := range lineEvents {
		logger.Infof("Processing event: %s\n", util.AnyToJson(event))

		switch event.Type {
		case linebot.EventTypeMessage:
			// TODO: revise quick reply
			// DEBUG: test handle
			response, err := line.SendQuickReply(event.ReplyToken)
			if err != nil {
				logger.Error("Error sending message:", err)
				return events.LambdaFunctionURLResponse{
					StatusCode: 500,
					Body:       fmt.Sprintf(`{"error": "Failed to send message: %s"}`, err),
				}, nil
			}
			logger.Debug("Response:", response)

		case linebot.EventTypeFollow:
			logger.Info("Received Follow event")
			logger.Info("Follow event User ID", event.Source)

			// if not exists, create new user in DB
			user := model.NewUser(event.Source.UserID, event.Timestamp)
			err := dao.CreateUser(user)
			if err != nil {
				if err != nil {
					if userAlreadyExistErr, ok := err.(*exception.UserAlreadyExistException); ok {
						logger.Error(userAlreadyExistErr.Error())
					} else {
						logger.Error("Error creating user:", err)

						return events.LambdaFunctionURLResponse{
							StatusCode: 500,
							Body:       fmt.Sprintf(`{"error": "Failed to create user: %s"}`, err),
						}, nil
					}
				}
			} else {
				logger.Info("Created new user:", user)
			}

		default:
			logger.Info("Unhandled event type: ", event.Type)
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
func handleHealthCheckCall(request events.LambdaFunctionURLRequest) bool {
	var body map[string]interface{}
	err := json.Unmarshal([]byte(request.Body), &body)
	if err != nil {
		log.Println(err)
		return true
	}

	events, ok := body["events"].([]interface{})
	if !ok || len(events) == 0 {
		log.Println("Request doesn't include any events. Likely a health check call.")
		return true
	}

	return false
}

func main() {
	lambda.Start(handleRequest)
}
