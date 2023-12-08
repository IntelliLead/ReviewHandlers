package postbackEvent

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/CoreCommonUtil/stringUtil"
    "github.com/IntelliLead/CoreDataAccess/ddbDao"
    "github.com/IntelliLead/CoreDataAccess/ddbDao/dbModel"
    "github.com/IntelliLead/CoreDataAccess/ddbDao/enum"
    "github.com/IntelliLead/CoreDataAccess/model"
    "github.com/IntelliLead/CoreDataAccess/model/type/bid"
    "github.com/IntelliLead/CoreDataAccess/model/type/rid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/aiUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "go.uber.org/zap"
)

// handleAutoQuickReplyToggle handles the auto quick reply toggle postback event
// returns autoQuickReplyEnabled, quickReplyMessage ,error
func handleAutoQuickReplyToggle(
    user model.User,
    businessId bid.BusinessId,
    businessDao *ddbDao.BusinessDao,
    log *zap.SugaredLogger,
) (model.Business, error) {
    userId := user.UserId

    businessPtr, err := businessDao.GetBusiness(businessId)
    if err != nil {
        log.Errorf("Error getting business by businessId '%s' during handling auto quick reply toggle: %s", businessId, err)
        return model.Business{}, err
    }
    if businessPtr == nil {
        errStr := fmt.Sprintf("Business not found for businessId: %s", businessId)
        log.Error(errStr)
        return model.Business{}, errors.New(errStr)
    }
    business := *businessPtr

    if !business.AutoQuickReplyEnabled && (stringUtil.IsEmptyStringPtr(business.QuickReplyMessage)) {
        return business, exception.NewAutoQuickReplyConditionNotMetException("Please fill in quick reply message before enabling auto quick reply")
    }

    attributeAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "autoQuickReplyEnabled", !business.AutoQuickReplyEnabled)
    if err != nil {
        return business, err
    }
    updatedBusiness, err := businessDao.UpdateAttributes(business.BusinessId, []dbModel.AttributeAction{attributeAction}, userId)
    if err != nil {
        return business, err
    }

    return updatedBusiness, nil
}

func handleGenerateAiReply(
    replyToken string,
    user model.User,
    businessId bid.BusinessId,
    reviewId rid.ReviewId,
    businessDao *ddbDao.BusinessDao,
    userDao *ddbDao.UserDao,
    reviewDao *ddbDao.ReviewDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger,
    gptApiKey string,
) error {
    userId := user.UserId

    // --------------------
    // Get business and review and perform validation
    // --------------------
    businessPtr, err := businessDao.GetBusiness(businessId)
    if err != nil {
        log.Errorf("Error getting business by businessId '%s' during handling generate AI reply: %s", businessId, err)
        return err
    }
    if businessPtr == nil {
        errStr := fmt.Sprintf("Business not found for businessId: %s", businessId)
        log.Error(errStr)
        return errors.New(errStr)
    }
    business := *businessPtr

    reviewPtr, err := reviewDao.GetReview(businessId.String(), reviewId)
    if err != nil {
        log.Errorf("Error getting review by businessId '%s' reviewId '%s' during handling generate AI reply: %s", businessId, reviewId.String(), err)
        return err
    }
    if reviewPtr == nil {
        errStr := fmt.Sprintf("Review not found for businessId: %s ; UserReviewId: %s", businessId, reviewId)
        log.Error(errStr)
        return errors.New(errStr)
    }
    review := *reviewPtr
    if stringUtil.IsEmptyStringPtr(review.Review) {
        errStr := fmt.Sprintf("Review is empty. Cannot generate AI reply. userId: %s ; UserReviewId: %s", userId, reviewId)
        log.Error(errStr)
        return errors.New(errStr)
    }

    // --------------------
    // Notify user that AI is generating reply
    // --------------------
    _, err = line.NotifyUserAiReplyGenerationInProgress(replyToken)
    if err != nil {
        log.Errorf("Error notifying user '%s' that AI is generating reply. Porceeding: %v", userId, err)
        return err
    }

    // --------------------
    // invoke gpt4
    // --------------------
    aiReply, err := aiUtil.NewAi(log, gptApiKey).GenerateReply(*review.Review, business, user)
    if err != nil {
        log.Errorf("Error invoking GPT to generate AI reply: %v", err)
        return err
    }

    // --------------------
    // create AI generated result card
    // --------------------
    generateAuthorName := user.LineUsername
    if stringUtil.IsEmptyString(generateAuthorName) {
        generateAuthorName = "您的同仁"
    }
    err = line.SendAiGeneratedReply(aiReply, review, generateAuthorName, business, user, userDao)
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

    if !user.SignatureEnabled && stringUtil.IsEmptyStringPtr(user.Signature) {
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
    if !business.KeywordEnabled && (stringUtil.IsEmptyStringPtr(business.Keywords) || stringUtil.IsEmptyStringPtr(business.BusinessDescription)) {
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
    businessDescription *string,
    userDao *ddbDao.UserDao,
    log *zap.SugaredLogger) (model.User, error) {
    if !user.ServiceRecommendationEnabled && stringUtil.IsEmptyStringPtr(user.ServiceRecommendation) && stringUtil.IsEmptyStringPtr(businessDescription) {
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
