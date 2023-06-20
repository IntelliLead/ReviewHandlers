package messageEvent

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

func ProcessReviewReplyMessage(
    userId string,
    event *linebot.Event,
    reviewDao *ddbDao.ReviewDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) (events.LambdaFunctionURLResponse, error) {

    textMessage := event.Message.(*linebot.TextMessage)
    message := textMessage.Text

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

    // send LINE message
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

    log.Infof("Successfully handled review reply event for user ID '%s', reply '%s' for review ID '%s'",
        userId, jsonUtil.AnyToJson(replyMessage), review.ReviewId.String())

    return events.LambdaFunctionURLResponse{
        StatusCode: 200,
        Body:       fmt.Sprintf(`{"message": "Successfully handled review reply event for user ID '%s'"}`, userId),
    }, nil

}
