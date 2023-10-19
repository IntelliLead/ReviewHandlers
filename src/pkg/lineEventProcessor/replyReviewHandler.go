package lineEventProcessor

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/zapierUtil"
    zapierModel "github.com/IntelliLead/ReviewHandlers/src/pkg/zapierUtil/model"
    "go.uber.org/zap"
    "time"
)

func ReplyReview(
    businessId string,
    userId string,
    replyMessage string,
    review model.Review,
    reviewDao *ddbDao.ReviewDao,
    log *zap.SugaredLogger) error {
    // post reply to zapier
    // --------------------
    zapier := zapierUtil.NewZapier(log)
    zapierEvent := zapierModel.ReplyToZapierEvent{
        VendorReviewId: review.VendorReviewId,
        Message:        replyMessage,
    }

    err := zapier.SendReplyEvent(review.ZapierReplyWebhook, zapierEvent)
    if err != nil {
        log.Errorf("Error sending reply event to Zapier for review %s from user '%s' of business '%s': %v", replyMessage, userId, businessId, err)
        return err
    }

    log.Infof("Sent reply event '%s' to Zapier from user '%s' of business '%s'", jsonUtil.AnyToJson(zapierEvent), userId, businessId)

    // update DDB
    // --------------------
    err = reviewDao.UpdateReview(ddbDao.UpdateReviewInput{
        BusinessId:  businessId,
        ReviewId:    *review.ReviewId,
        LastUpdated: time.Now(),
        LastReplied: time.Now(),
        Reply:       replyMessage,
        RepliedBy:   userId,
    })
    if err != nil {
        log.Errorf("Error updating review '%s' from user '%s': %v", review.ReviewId, userId, err)
        return err
    }

    return nil
}
