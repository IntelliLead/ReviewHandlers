package lineEventProcessor

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/zapierUtil"
    zapierModel "github.com/IntelliLead/ReviewHandlers/src/pkg/zapierUtil/model"
    "github.com/aws/aws-lambda-go/events"
    "go.uber.org/zap"
    "time"
)

func ReplyReview(
    userId string,
    replyToken *string,
    replyMessage string,
    review model.Review,
    reviewDao *ddbDao.ReviewDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger,
    isAutoReply bool) (events.LambdaFunctionURLResponse, error) {
    // post reply to zapier
    // --------------------
    zapier := zapierUtil.NewZapier(log)
    zapierEvent := zapierModel.ReplyToZapierEvent{
        VendorReviewId: review.VendorReviewId,
        Message:        replyMessage,
    }

    err := zapier.SendReplyEvent(review.ZapierReplyWebhook, zapierEvent)
    if err != nil {
        log.Errorf("Error sending reply event to Zapier for review %s from user '%s': %v", replyMessage, userId, err)

        var notifyUserReplyProcessedErr error
        if util.IsEmptyStringPtr(replyToken) {
            _, notifyUserReplyProcessedErr = line.NotifyUserReplyProcessed(userId, false, review.ReviewerName, isAutoReply)
        } else {
            _, notifyUserReplyProcessedErr = line.ReplyUserReplyProcessed(*replyToken, false, review.ReviewerName, isAutoReply)
        }
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
        BusinessId:  userId,
        ReviewId:    *review.ReviewId,
        LastUpdated: time.Now(),
        LastReplied: time.Now(),
        Reply:       replyMessage,
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
    if util.IsEmptyStringPtr(replyToken) {
        _, err = line.NotifyUserReplyProcessed(userId, true, review.ReviewerName, isAutoReply)
    } else {
        _, err = line.ReplyUserReplyProcessed(*replyToken, true, review.ReviewerName, isAutoReply)
    }
    if err != nil {
        log.Errorf("Error notifying reply processed successfully to user '%s' for review '%s': %v",
            userId, jsonUtil.AnyToJson(review), err)

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to notify user '%s' review processed successfully: %v"}`, userId, err),
        }, err
    }

    log.Infof("Successfully notified user '%s' reply for review ID '%s' was processed",
        userId, jsonUtil.AnyToJson(review))

    return events.LambdaFunctionURLResponse{
        StatusCode: 200,
        Body:       fmt.Sprintf(`{"message": "Successfully handled review reply event for user ID '%s'"}`, userId),
    }, nil
}
