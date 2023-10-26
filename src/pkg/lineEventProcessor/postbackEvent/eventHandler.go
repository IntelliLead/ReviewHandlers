package postbackEvent

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/aiUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/dbModel"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "go.uber.org/zap"
)

// handleAutoQuickReplyToggle handles the auto quick reply toggle postback event
// returns autoQuickReplyEnabled, quickReplyMessage ,error
func handleAutoQuickReplyToggle(
    user model.User,
    business model.Business,
    businessDao *ddbDao.BusinessDao,
) (bool, *string, error) {
    userId := user.UserId
    if !business.AutoQuickReplyEnabled && (util.IsEmptyStringPtr(business.QuickReplyMessage)) {
        return false, nil, exception.NewAutoQuickReplyConditionNotMetException("Please fill in quick reply message before enabling auto quick reply")
    }

    attributeAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "autoQuickReplyEnabled", !business.AutoQuickReplyEnabled)
    if err != nil {
        return false, nil, err
    }
    updatedBusiness, err := businessDao.UpdateAttributes(business.BusinessId, []dbModel.AttributeAction{attributeAction}, userId)
    if err != nil {
        return false, nil, err
    }
    return updatedBusiness.AutoQuickReplyEnabled, updatedBusiness.QuickReplyMessage, nil

}

func handleGenerateAiReply(
    replyToken string,
    user model.User,
    business model.Business,
    reviewId _type.ReviewId,
    reviewDao *ddbDao.ReviewDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) error {
    userId := user.UserId
    // Notify user that AI is generating reply
    _, err := line.NotifyUserAiReplyGenerationInProgress(replyToken)
    if err != nil {
        log.Errorf("Error notifying user '%s' that AI is generating reply. Porceeding: %v", userId, err)
        return err
    }

    review, err := reviewDao.GetReview(business.BusinessId, reviewId)
    if err != nil {
        log.Errorf("Error getting review by businessId '%s' reviewId '%s' during handling generate AI reply: %s", business.BusinessId, reviewId.String(), err)
        return err
    }
    if review == nil {
        errStr := fmt.Sprintf("Review not found for businessId: %s ; ReviewId: %s", business.BusinessId, reviewId)
        log.Error(errStr)

        // TODO: [INT-91] remove this after all users have been backfilled
        log.Infof("[Fallback] Trying to find review by userId '%s' reviewId '%s' during handling generate AI reply", userId, reviewId)

        review, err = reviewDao.GetReview(user.UserId, reviewId)
        if err != nil {
            log.Errorf("Error getting review by userId '%s' reviewId '%s' during handling generate AI reply: %s", userId, reviewId.String(), err)
            return err
        }
        if review == nil {
            errStr := fmt.Sprintf("Review not found for userId: %s ; ReviewId: %s", userId, reviewId)
            log.Error(errStr)
            return errors.New(errStr)
        }
    }

    // invoke gpt4
    if util.IsEmptyStringPtr(review.Review) {
        errStr := fmt.Sprintf("Review is empty. Cannot generate AI reply. userId: %s ; ReviewId: %s", userId, reviewId)
        log.Error(errStr)
        return errors.New(errStr)
    }
    aiReply, err := aiUtil.NewAi(log).GenerateReply(*review.Review, business, user)
    if err != nil {
        log.Errorf("Error invoking GPT to generate AI reply: %v", err)
        return err
    }

    // create AI generated result card
    generateAuthorName := user.LineUsername
    if util.IsEmptyString(generateAuthorName) {
        generateAuthorName = "您的同仁"
    }
    err = line.SendAiGeneratedReply(aiReply, *review, business.UserIds, generateAuthorName)
    if err != nil {
        log.Errorf("Error sending AI generated reply to user '%s': %v", userId, err)
        return err
    }

    return nil
}

// handleEmojiToggle handles the emoji toggle postback event
// replyToken is only used when there is an error
func handleEmojiToggle(
    user model.User,
    userDao *ddbDao.UserDao,
    log *zap.SugaredLogger) (model.User, error) {
    action, err := dbModel.NewAttributeAction(enum.ActionUpdate, "emojiEnabled", !user.EmojiEnabled)
    if err != nil {
        return model.User{}, err
    }
    updatedUser, err := userDao.UpdateAttributes(user.UserId, []dbModel.AttributeAction{action})
    if err != nil {
        log.Errorf("Error updating emoji enabled to %v for user '%s': %v", !user.EmojiEnabled, user.UserId, err)

        return model.User{}, err
    }

    return updatedUser, nil
}

func handleSignatureToggle(
    user model.User,
    userDao *ddbDao.UserDao,
    log *zap.SugaredLogger) (model.User, error) {

    if !user.SignatureEnabled && util.IsEmptyStringPtr(user.Signature) {
        return model.User{}, exception.NewSignatureDoesNotExistException("Signature does not exist for " + user.UserId)
    }

    action, err := dbModel.NewAttributeAction(enum.ActionUpdate, "signatureEnabled", !user.SignatureEnabled)
    if err != nil {
        return model.User{}, err
    }
    updatedUser, err := userDao.UpdateAttributes(user.UserId, []dbModel.AttributeAction{action})
    if err != nil {
        log.Errorf("Error updating signature enabled to %v for user '%s': %v", !user.SignatureEnabled, user.UserId, err)

        return model.User{}, err
    }

    return updatedUser, nil
}

func handleKeywordToggle(
    user model.User,
    business model.Business,
    businessDao *ddbDao.BusinessDao,
) (model.Business, error) {
    if !business.KeywordEnabled && (util.IsEmptyStringPtr(business.Keywords) || util.IsEmptyStringPtr(business.BusinessDescription)) {
        return model.Business{}, exception.NewKeywordConditionNotMetException("Keyword condition not met for " + user.UserId)
    }

    action, err := dbModel.NewAttributeAction(enum.ActionUpdate, "keywordEnabled", !business.KeywordEnabled)
    if err != nil {
        return model.Business{}, err
    }
    updatedBusiness, err := businessDao.UpdateAttributes(business.BusinessId, []dbModel.AttributeAction{action}, user.UserId)
    if err != nil {
        return model.Business{}, err
    }

    return updatedBusiness, nil
}

func handleServiceRecommendationToggle(
    user model.User,
    userDao *ddbDao.UserDao,
    log *zap.SugaredLogger) (model.User, error) {

    if !user.ServiceRecommendationEnabled && util.IsEmptyStringPtr(user.ServiceRecommendation) && util.IsEmptyStringPtr(user.BusinessDescription) {
        return model.User{}, exception.NewServiceRecommendationConditionNotMetException("Service recommendation condition not met for " + user.UserId)
    }

    var updatedUser model.User
    var err error

    action, err := dbModel.NewAttributeAction(enum.ActionUpdate, "serviceRecommendationEnabled", !user.ServiceRecommendationEnabled)
    if err != nil {
        return model.User{}, err
    }
    updatedUser, err = userDao.UpdateAttributes(user.UserId, []dbModel.AttributeAction{action})
    if err != nil {
        log.Errorf("Error updating service recommendation enabled to %v for user '%s': %v", !user.ServiceRecommendationEnabled, user.UserId, err)

        return model.User{}, err
    }

    return updatedUser, nil
}
