package lineEventProcessor

import (
    "github.com/IntelliLead/CoreCommonUtil/jsonUtil"
    "github.com/IntelliLead/CoreDataAccess/ddbDao"
    "github.com/IntelliLead/CoreDataAccess/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/zapierUtil"
    model2 "github.com/IntelliLead/ReviewHandlers/src/pkg/zapierUtil/model"
    "go.uber.org/zap"
    "time"
)

func ReplyReview(
    repliedByUserId string,
    replyMessage string,
    review model.Review,
    reviewDao *ddbDao.ReviewDao,
    log *zap.SugaredLogger) error {
    if review.ZapierReplyWebhook == util.TestZapierReplyWebhook {
        log.Infof("Skipping reply event to Zapier for review %s from user '%s' of business '%s' because it is a test webhook", replyMessage, repliedByUserId, review.BusinessId)
    } else {
        // post reply to zapier
        // --------------------
        zapier := zapierUtil.NewZapier(log)
        zapierEvent := model2.ReplyToZapierEvent{
            VendorReviewId: review.VendorReviewId,
            Message:        replyMessage,
        }

        err := zapier.SendReplyEvent(review.ZapierReplyWebhook, zapierEvent)
        if err != nil {
            log.Errorf("Error sending reply event to Zapier for review %s from user '%s' of business '%s': %v", replyMessage, repliedByUserId, review.BusinessId, err)
            return err
        }

        log.Infof("Sent reply event '%s' to Zapier from user '%s' of business '%s'", jsonUtil.AnyToJson(zapierEvent), repliedByUserId, review.BusinessId)
    }

    // update DDB
    // --------------------
    err := reviewDao.UpdateReview(ddbDao.UpdateReviewInput{
        BusinessId:  review.BusinessId,
        ReviewId:    review.ReviewId,
        LastUpdated: time.Now(),
        LastReplied: time.Now(),
        Reply:       replyMessage,
        RepliedBy:   repliedByUserId,
    })
    if err != nil {
        log.Errorf("Error updating review '%s' from user '%s': %v", review.ReviewId, repliedByUserId, err)
        return err
    }

    return nil
}
