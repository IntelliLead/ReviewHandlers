package lineEventProcessor

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/zapierUtil"
    zapierModel "github.com/IntelliLead/ReviewHandlers/src/pkg/zapierUtil/model"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
    "time"
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

    // process help request
    // --------------------------------
    if lineUtil.IsHelpMessage(message) {
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
    }

    // process update quick reply message request
    // --------------------------------
    if lineUtil.IsUpdateQuickReplyMessage(message) {
        cmdMsg := lineUtil.ParseCommandMessage(message, false)
        quickReplyMessage := cmdMsg.Args[0]

        // update DDB
        updatedUser, err := userDao.UpdateQuickReplyMessage(userId, quickReplyMessage)
        if err != nil {
            log.Errorf("Error updating quick reply message '%s' for user '%s': %v", quickReplyMessage, userId, err)

            _, err := line.NotifyUserUpdateQuickReplyMessageFailed(event.ReplyToken)
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

        err = line.ShowQuickReplySettings(updatedUser, event.ReplyToken, true)
        if err != nil {
            log.Errorf("Error showing quick reply settings for user '%s': %v", userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to show quick reply settings: %s"}`, err),
            }, err
        }

        log.Infof("Successfully processed update quick reply message request to user '%s'", userId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Successfully processed help request"}`,
        }, nil
    }

    // process review reply request
    // --------------------------------
    if lineUtil.IsReviewReplyMessage(message) {
        replyMessage, err := lineUtil.ParseReplyMessage(message)
        if err != nil {
            log.Error("Error parsing reply message:", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to parse reply message: %s"}`, err),
            }, err
        }

        // fetch review from DDB
        // --------------------
        review, err := reviewDao.GetReview(userId, replyMessage.ReviewId)
        if err != nil {
            log.Errorf("Error getting review for review reply %s from user '%s': %v", replyMessage, userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to get review: %s"}`, err),
            }, err
        }

        log.Debug("Got Review:", jsonUtil.AnyToJson(review))

        // validate message does not contain LINE emojis
        // --------------------------------
        emojis := textMessage.Emojis
        if len(emojis) > 0 {
            _, err = line.NotifyUserReplyProcessedWithReason(event.ReplyToken, false, review.ReviewerName,
                "暫時不支援LINE Emoji，但是您可以考慮使用 Unicode emoji （比如👍🏻）。️很抱歉為您造成不便。")

            return events.LambdaFunctionURLResponse{
                StatusCode: 200,
                Body:       `{"message": "Notified Emoji not yet supported"}`,
            }, nil
        }

        // post reply to zapier
        // --------------------
        zapier := zapierUtil.NewZapier(log)
        zapierEvent := zapierModel.ReplyToZapierEvent{
            VendorReviewId: review.VendorReviewId,
            Message:        replyMessage.Message,
        }

        err = zapier.SendReplyEvent(review.ZapierReplyWebhook, zapierEvent)
        if err != nil {
            log.Errorf("Error sending reply event to Zapier for review %s from user '%s': %v", replyMessage, userId, err)

            _, notifyUserReplyProcessedErr := line.NotifyUserReplyProcessed(event.ReplyToken, false, review.ReviewerName)
            if notifyUserReplyProcessedErr != nil {
                log.Errorf("Error notifying reply failure for user '%s' for review '%s' with ID '%s': %v",
                    userId, jsonUtil.AnyToJson(replyMessage), review.ReviewId.String(), err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to notify reply failure for user '%s' : %v. Reply Failure reason: %v"}`, userId, notifyUserReplyProcessedErr, err),
                }, notifyUserReplyProcessedErr
            }

            log.Infof("Successfully notified user '%s' reply '%s' for review ID '%s' was NOT processed",
                userId, jsonUtil.AnyToJson(replyMessage), review.ReviewId.String())

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to send reply event to Zapier: %s"}`, err),
            }, err
        }

        log.Infof("Sent reply event '%s' to Zapier from user '%s'", jsonUtil.AnyToJson(zapierEvent), userId)

        // reply LINE message
        // --------------------
        _, err = line.NotifyUserReplyProcessed(event.ReplyToken, true, review.ReviewerName)
        if err != nil {
            log.Errorf("Error notifying user '%s' for review '%s' with ID '%s': %v",
                userId, jsonUtil.AnyToJson(replyMessage), review.ReviewId.String(), err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to notify user '%s' : %v"}`, userId, err),
            }, err
        }

        log.Infof("Successfully notified user '%s' reply '%s' for review ID '%s' was processed",
            userId, jsonUtil.AnyToJson(replyMessage), review.ReviewId.String())

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
            log.Errorf("Error updating review '%s' from user '%s': %v", review.ReviewId, userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to update review DB record: %s"}`, err),
            }, err
        }

        log.Infof("Successfully handled review reply event for user ID '%s', reply '%s' for review ID '%s'",
            userId, jsonUtil.AnyToJson(replyMessage), review.ReviewId.String())

        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       fmt.Sprintf(`{"message": "Successfully handled review reply event for user ID '%s'"}`, userId),
        }, nil
    }

    // handle unknown message from user
    // --------------------
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
