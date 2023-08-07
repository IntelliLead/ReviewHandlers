package lineEventProcessor

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineEventProcessor/messageEvent"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
)

// ProcessMessageEvent processes a message event from LINE
// It returns a LambdaFunctionURLResponse and an error
func ProcessMessageEvent(event *linebot.Event,
    userId string,
    userDao *ddbDao.UserDao,
    reviewDao *ddbDao.ReviewDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) (events.LambdaFunctionURLResponse, error) {

    // validate is message from user
    // --------------------------------
    isMessageFromUser := lineUtil.IsMessageFromUser(event)
    if !isMessageFromUser {
        log.Info("Not a message from user. Ignoring.")
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Not a text message from user. Ignoring."}`,
        }, nil
    }

    // validate is text message from user
    // --------------------------------
    isTextMessageFromUser, err := lineUtil.IsTextMessage(event)
    if err != nil {
        log.Error("Error checking if event is text message from user:", err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to check if event is text message from user: %s"}`, err),
        }, err
    }

    if !isTextMessageFromUser {
        log.Info("Message from user is not a text message.")

        err := line.SendUnknownResponseReply(event.ReplyToken)
        if err != nil {
            log.Error("Error executing SendUnknownResponseReply: ", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error executing SendUnknownResponseReply: %s"}`, err),
            }, err
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Message from user is not a text message."}`,
        }, nil
    }

    textMessage := event.Message.(*linebot.TextMessage)
    message := textMessage.Text
    log.Infof("Received text message from user '%s': %s", userId, message)

    // process review reply request
    // --------------------------------
    if lineUtil.IsReviewReplyMessage(message) {
        return messageEvent.ProcessReviewReplyMessage(userId, event, reviewDao, line, log)
    }

    // process command requests
    cmdMsg := lineUtil.ParseCommandMessage(message, false)
    args := cmdMsg.Args[0]
    switch cmdMsg.Command {
    case "h", "Help", "help", "幫助", "協助":
        _, err := line.ReplyHelpMessage(event.ReplyToken)
        if err != nil {
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to reply help message: %s"}`, err),
            }, err
        }

        log.Infof("Successfully processed help request to user '%s'", userId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Successfully processed help request"}`,
        }, nil

    case "q", util.UpdateQuickReplyMessageCmd, "快速回復":
        // process update quick reply message request
        quickReplyMessage := args

        // update DDB
        updatedUser, err := userDao.UpdateAttribute(userId, "quickReplyMessage", quickReplyMessage)
        if err != nil {
            log.Errorf("Error updating quick reply message '%s' for user '%s': %v", quickReplyMessage, userId, err)

            _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "快速回覆訊息")
            if err != nil {
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to notify user of update quick reply message failed: %s"}`, err),
                }, err
            }
            log.Error("Successfully notified user of update quick reply message failed")

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to update quick reply message: %s"}`, err),
            }, err
        }

        err = line.ShowQuickReplySettings(event.ReplyToken, updatedUser, true)
        if err != nil {
            log.Errorf("Error showing quick reply settings for user '%s': %v", userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to show quick reply settings: %s"}`, err),
            }, err
        }

        log.Infof("Successfully processed update quick reply message request for user '%s'", userId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Successfully processed update quick reply message request"}`,
        }, nil

    case "d", util.UpdateBusinessDescriptionMessageCmd, "主要業務":
        // process update quick reply message request
        businessDescription := args

        // update DDB
        var updatedUser model.User
        var err error
        if util.IsEmptyString(businessDescription) {
            updatedUser, err = userDao.DeleteAttribute(userId, "businessDescription")
        } else {
            updatedUser, err = userDao.UpdateAttribute(userId, "businessDescription", businessDescription)
        }
        if err != nil {
            log.Errorf("Error updating business description '%s' for user '%s': %v", businessDescription, userId, err)

            _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "主要業務")
            if err != nil {
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to notify user of update business description failed: %s"}`, err),
                }, err
            }
            log.Error("Successfully notified user of update business description failed")

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to update business description: %s"}`, err),
            }, err
        }

        err = line.ShowSeoSettings(event.ReplyToken, updatedUser)
        if err != nil {
            log.Errorf("Error showing seo settings for user '%s': %v", userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to show seo settings: %s"}`, err),
            }, err
        }

        log.Infof("Successfully processed update business description request for user '%s'", userId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Successfully processed update business description request"}`,
        }, nil

    case "k", util.UpdateKeywordsMessageCmd, "關鍵字":
        // process update quick reply message request
        keywords := args

        // update DDB
        var updatedUser model.User
        var err error
        if util.IsEmptyString(keywords) {
            updatedUser, err = userDao.DeleteAttribute(userId, "keywords")
        } else {
            updatedUser, err = userDao.UpdateAttribute(userId, "keywords", keywords)
        }
        if err != nil {
            log.Errorf("Error updating keywords '%s' for user '%s': %v", keywords, userId, err)

            _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "關鍵字")
            if err != nil {
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to notify user of update keywords failed: %s"}`, err),
                }, err
            }
            log.Error("Successfully notified user of update keywords failed")

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to update keywords: %s"}`, err),
            }, err
        }

        err = line.ShowSeoSettings(event.ReplyToken, updatedUser)
        if err != nil {
            log.Errorf("Error showing seo settings for user '%s': %v", userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to show seo settings: %s"}`, err),
            }, err
        }

        log.Infof("Successfully processed update keywords request for user '%s'", userId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Successfully processed update keywords request"}`,
        }, nil

    default:
        // handle unknown message from user
        err = line.SendUnknownResponseReply(event.ReplyToken)
        if err != nil {
            log.Error("Error executing SendUnknownResponseReply: ", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error executing SendUnknownResponseReply: %s"}`, err),
            }, err
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Text message from user is not handled."}`,
        }, nil

    }

}
