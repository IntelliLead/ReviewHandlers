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
    "github.com/IntelliLead/ReviewHandlers/src/pkg/zapierUtil"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
    "time"
)

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    log := logger.NewLogger()

    log.Info("Received new request: ", util.AnyToJson(request))

    // --------------------
    // Check if the request is a health check call
    // --------------------
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

    // --------------------
    // initialize resources
    // --------------------
    // DDB
    mySession := session.Must(session.NewSession())
    userDao := ddbDao.NewUserDao(dynamodb.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1")), log)
    reviewDao := ddbDao.NewReviewDao(dynamodb.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1")), log)

    // LINE
    line := lineUtil.NewLine(log)

    // parse message to LINE events
    var lineEvents []*linebot.Event
    // TODO: handle local differently if needed
    // if os.Getenv("Stage") == enum.StageLocal.String() {
    //     log.Debug("Running in local environment. Skipping LINE event parser")
    //     lineEvents = lineEventsHandlerTestEvents.TestReplyEvent
    // } else {
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
    // }
    log.Infof("Received %d LINE events: ", len(lineEvents))

    // --------------------
    // process LINE events
    // --------------------
    for _, event := range lineEvents {
        log.Infof("Processing event: %s\n", util.AnyToJson(event))

        userId := event.Source.UserID

        switch event.Type {
        case linebot.EventTypeMessage:
            // validate is review reply message
            // --------------------------------

            isTextMsgFromUser, err := lineUtil.IsTextMessageFromUser(event)
            if err != nil {
                log.Error("Error checking if event is text message from user:", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to check if event is text message from user: %s"}`, err),
                }, nil
            }
            if isTextMsgFromUser {
                textMessage := event.Message.(*linebot.TextMessage)

                // check if message contain emoji
                emojis := textMessage.Emojis
                if len(emojis) > 0 {
                    // TODO: Reply user: reply emojis not yet supported

                    return events.LambdaFunctionURLResponse{
                        StatusCode: 200,
                        Body:       `{"message": "Emoji not yet supported"}`,
                    }, nil
                }

                message := textMessage.Text
                log.Infof("Received text message from user '%s': %s", userId, message)

                if lineUtil.IsReviewReplyMessage(message) {
                    replyMessage, err := lineUtil.ParseReplyMessage(message)
                    if err != nil {
                        log.Error("Error parsing reply message:", err)
                    }

                    // fetch review from DDB
                    // --------------------
                    review, err := reviewDao.GetReview(userId, replyMessage.ReviewId)
                    if err != nil {
                        log.Errorf("Error getting review for review reply %s from user '%s': %v", replyMessage, userId, err)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Failed to get review: %s"}`, err),
                        }, nil
                    }

                    log.Debug("Got Review:", util.AnyToJson(review))

                    // post reply to zapier
                    // --------------------
                    zapier := zapierUtil.NewZapier(log)
                    zapierEvent := zapierUtil.ReplyToZapierEvent{
                        VendorReviewId: review.VendorReviewId,
                        Message:        replyMessage.Message,
                    }

                    err = zapier.SendReplyEvent(review.ZapierReplyWebhook, zapierEvent)
                    if err != nil {
                        log.Errorf("Error sending reply event to Zapier for review %s from user '%s': %v", replyMessage, userId, err)

                        _, err = line.NotifyUserReplyProcessed(event.ReplyToken, false, review.ReviewerName)
                        if err != nil {
                            log.Errorf("Error notifying reply failure for user '%s' for review '%s' with ID '%s': %v",
                                userId, util.AnyToJson(replyMessage), review.ReviewId.String(), err)
                            return events.LambdaFunctionURLResponse{
                                StatusCode: 500,
                                Body:       fmt.Sprintf(`{"error": "Failed to notify reply failure for user '%s' : %v"}`, userId, err),
                            }, err
                        }

                        log.Infof("Successfully notified user '%s' reply '%s' for review ID '%s' was NOT processed",
                            userId, util.AnyToJson(replyMessage), review.ReviewId.String())

                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Failed to send reply event to Zapier: %s"}`, err),
                        }, nil
                    }

                    log.Infof("Sent reply event '%s' to Zapier from user '%s'", util.AnyToJson(zapierEvent), userId)

                    // reply LINE message
                    // --------------------
                    _, err = line.NotifyUserReplyProcessed(event.ReplyToken, true, review.ReviewerName)
                    if err != nil {
                        log.Errorf("Error notifying user '%s' for review '%s' with ID '%s': %v",
                            userId, util.AnyToJson(replyMessage), review.ReviewId.String(), err)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Failed to notify user '%s' : %v"}`, userId, err),
                        }, err
                    }

                    log.Infof("Successfully notified user '%s' reply '%s' for review ID '%s' was processed",
                        userId, util.AnyToJson(replyMessage), review.ReviewId.String())

                    // update DDB
                    // --------------------
                    err = reviewDao.UpdateReview(ddbDao.UpdateReviewInput{
                        UserId:      userId,
                        ReviewId:    *review.ReviewId,
                        LastUpdated: time.Now(),
                        LastReplied: time.Now(),
                        Reply:       replyMessage.Message,
                    })
                    if err != nil {
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Failed to update review DB record: %s"}`, err),
                        }, err
                    }

                    log.Infof("Completed handling review reply event for user ID '%s', reply '%s' for review ID '%s'",
                        userId, util.AnyToJson(replyMessage), review.ReviewId.String())
                }

                // TODO: handle other type of messages such as help, etc.
            }

        case linebot.EventTypeFollow:
            log.Info("Received Follow event")
            log.Info("Follow event source: ", util.AnyToJson(event.Source))

            // if not exists, create new user in DB
            user := model.NewUser(event.Source.UserID, event.Timestamp)
            err := userDao.CreateUser(user)
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
