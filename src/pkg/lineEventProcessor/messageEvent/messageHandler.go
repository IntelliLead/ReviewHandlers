package messageEvent

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/dbModel"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "go.uber.org/zap"
)

func buildQuickReplyUpdateAttributeActions(quickReplyMessage string) ([]dbModel.AttributeAction, error) {
    if util.IsEmptyString(quickReplyMessage) {
        removeAction, err := dbModel.NewAttributeAction(enum.ActionRemove, "quickReplyMessage", nil)
        if err != nil {
            return nil, err
        }
        // disable depending features
        updateAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "autoQuickReplyEnabled", false)
        if err != nil {
            return nil, err
        }
        return []dbModel.AttributeAction{removeAction, updateAction}, nil
    } else {
        updateAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "quickReplyMessage", quickReplyMessage)
        if err != nil {
            return nil, err
        }
        return []dbModel.AttributeAction{updateAction}, nil
    }
}

// handleUpdateQuickReply handles the update of the quick reply message
// returns:
// 1. bool: whether the quick reply is enabled
// 2. string: the quick reply message
// 3. error
func handleUpdateQuickReply(
    user model.User,
    quickReplyMessage string,
    businessDao *ddbDao.BusinessDao,
    log *zap.SugaredLogger) (bool, *string, error) {
    userId := user.UserId
    actions, err := buildQuickReplyUpdateAttributeActions(quickReplyMessage)
    if err != nil {
        return false, nil, err
    }

    updatedBusiness, err := businessDao.UpdateAttributes(*user.ActiveBusinessId, actions, userId)
    if err != nil {
        log.Errorf("Error updating quick reply message '%s' for business '%s': %v", quickReplyMessage, *user.ActiveBusinessId, err)
        return false, nil, err
    }

    return updatedBusiness.AutoQuickReplyEnabled, updatedBusiness.QuickReplyMessage, nil

}

// handleUpdateKeywordEnabled handles the update of the keyword enabled
func handleBusinessDescriptionUpdate(
    user model.User,
    businessDescription string,
    userDao *ddbDao.UserDao,
    businessDao *ddbDao.BusinessDao,
    log *zap.SugaredLogger) (model.User, model.Business, error) {
    userId := user.UserId
    var updatedBusiness model.Business
    if util.IsEmptyString(businessDescription) {
        removeBusinessDescriptionAction, err := dbModel.NewAttributeAction(enum.ActionRemove, "businessDescription", nil)
        if err != nil {
            return model.User{}, model.Business{}, err
        }
        disableKeywordEnabledAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "keywordEnabled", false)
        if err != nil {
            return model.User{}, model.Business{}, err
        }

        attributeActions := []dbModel.AttributeAction{
            removeBusinessDescriptionAction,
            // disable depending features
            disableKeywordEnabledAction,
        }

        updatedBusiness, err = businessDao.UpdateAttributes(*user.ActiveBusinessId, attributeActions, userId)
        if err != nil {
            log.Errorf("Error updating business description '%s' for business '%s': %v", businessDescription, *user.ActiveBusinessId, err)
            return model.User{}, model.Business{}, err
        }

        // disable depending features
        if !util.IsEmptyStringPtr(user.ServiceRecommendation) {
            disableServiceRecommendationEnabledAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "serviceRecommendationEnabled", false)
            if err != nil {
                return model.User{}, model.Business{}, err
            }
            user, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{disableServiceRecommendationEnabledAction})
            if err != nil {
                log.Errorf("Error disabling service recommendation enabled '%s' for user '%s': %v", businessDescription, userId, err)
                return model.User{}, model.Business{}, err
            }
        }
    } else {
        attributeAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "businessDescription", businessDescription)
        if err != nil {
            return model.User{}, model.Business{}, err
        }

        updatedBusiness, err = businessDao.UpdateAttributes(*user.ActiveBusinessId, []dbModel.AttributeAction{attributeAction}, userId)
        if err != nil {
            log.Errorf("Error updating business description '%s' for business '%s': %v", businessDescription, *user.ActiveBusinessId, err)
            return model.User{}, model.Business{}, err
        }
    }

    log.Infof("Successfully processed update business description request for user '%s'", userId)

    return user, updatedBusiness, nil
}

func handleUpdateSignature(
    user model.User,
    signature string,
    userDao *ddbDao.UserDao,
    log *zap.SugaredLogger) (model.User, error) {
    userId := user.UserId

    var err error
    if util.IsEmptyString(signature) {
        user, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{
            {Action: enum.ActionRemove, Name: "signature"},
            // disable depending features
            {Action: enum.ActionUpdate, Name: "signatureEnabled", Value: false},
        })
    } else {
        user, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{
            {Action: enum.ActionUpdate, Name: "signature", Value: signature},
        })
    }
    if err != nil {
        log.Errorf("Error updating signature '%s' for user '%s': %v", signature, userId, err)
        return model.User{}, err
    }

    return user, nil
}

func handleUpdateKeywords(
    user model.User,
    keywords string,
    businessDao *ddbDao.BusinessDao,
    log *zap.SugaredLogger) (model.Business, error) {
    userId := user.UserId

    var updatedBusiness model.Business
    if util.IsEmptyString(keywords) {
        removeKeywordsAction, err := dbModel.NewAttributeAction(enum.ActionRemove, "keywords", nil)
        if err != nil {
            return model.Business{}, err
        }
        // disable depending features
        disableKeywordEnabledAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "keywordEnabled", false)
        if err != nil {
            return model.Business{}, err
        }

        updatedBusiness, err = businessDao.UpdateAttributes(*user.ActiveBusinessId, []dbModel.AttributeAction{removeKeywordsAction, disableKeywordEnabledAction}, userId)
        if err != nil {
            return model.Business{}, err
        }
    } else {
        updateKeywordsAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "keywords", keywords)
        if err != nil {
            return model.Business{}, err
        }

        updatedBusiness, err = businessDao.UpdateAttributes(*user.ActiveBusinessId, []dbModel.AttributeAction{updateKeywordsAction}, userId)
        if err != nil {
            return model.Business{}, err
        }
    }

    return updatedBusiness, nil
}

func handleUpdateServiceRecommendation(
    user model.User,
    serviceRecommendation string,
    userDao *ddbDao.UserDao,
) (model.User, error) {
    userId := user.UserId
    var updatedUser model.User
    if util.IsEmptyString(serviceRecommendation) {
        removeRecommendationAction, err := dbModel.NewAttributeAction(enum.ActionRemove, "serviceRecommendation", nil)
        if err != nil {
            return model.User{}, err
        }
        // disable depending features
        disableServiceRecommendationEnabledAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "serviceRecommendationEnabled", false)
        if err != nil {
            return model.User{}, err
        }

        updatedUser, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{
            removeRecommendationAction,
            // disable depending features
            disableServiceRecommendationEnabledAction,
        })
        if err != nil {
            return model.User{}, err
        }
    } else {
        action, err := dbModel.NewAttributeAction(enum.ActionUpdate, "serviceRecommendation", serviceRecommendation)
        if err != nil {
            return model.User{}, err
        }

        updatedUser, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{action})
        if err != nil {
            return model.User{}, err
        }
    }

    return updatedUser, nil
}
