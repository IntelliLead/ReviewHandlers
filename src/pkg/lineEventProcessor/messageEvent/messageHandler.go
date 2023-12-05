package messageEvent

import (
    "github.com/IntelliLead/CoreCommonUtil/stringUtil"
    "github.com/IntelliLead/CoreDataAccess/ddbDao"
    "github.com/IntelliLead/CoreDataAccess/ddbDao/dbModel"
    "github.com/IntelliLead/CoreDataAccess/ddbDao/enum"
    "github.com/IntelliLead/CoreDataAccess/model"
    "github.com/IntelliLead/CoreDataAccess/model/type/bid"
    "go.uber.org/zap"
)

func buildQuickReplyUpdateAttributeActions(quickReplyMessage string) ([]dbModel.AttributeAction, error) {
    if stringUtil.IsEmptyString(quickReplyMessage) {
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

// handleUpdateQuickReplyMessage handles the update of the quick reply message
// returns:
// 1. bool: whether the quick reply is enabled
// 2. string: quick reply message
// 3. error
func handleUpdateQuickReplyMessage(
    businessId bid.BusinessId,
    quickReplyMessage string,
    updatedByUserId string,
    businessDao *ddbDao.BusinessDao,
    log *zap.SugaredLogger) (model.Business, error) {
    actions, err := buildQuickReplyUpdateAttributeActions(quickReplyMessage)
    if err != nil {
        log.Errorf("Error building quick reply message update attribute actions: %v", err)
        return model.Business{}, err
    }

    business, err := businessDao.UpdateAttributes(businessId, actions, updatedByUserId)
    if err != nil {
        log.Errorf("Error updating quick reply message '%s' for business '%s': %v", quickReplyMessage, businessId, err)
        return model.Business{}, err
    }

    return business, nil
}

// handleBusinessDescriptionUpdate handles the update of the business description.
// returns:
// 1. updated user (if there is no operation on user, it will be the same as the input)
// 2. updated business
// 3. error
func handleBusinessDescriptionUpdate(
    businessId bid.BusinessId,
    businessDescription string,
    updateRequestUser model.User,
    userDao *ddbDao.UserDao,
    businessDao *ddbDao.BusinessDao,
    log *zap.SugaredLogger) (model.User, model.Business, error) {
    var updatedBusiness model.Business
    updatedUser := updateRequestUser
    if stringUtil.IsEmptyString(businessDescription) {
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

        updatedBusiness, err = businessDao.UpdateAttributes(businessId, attributeActions, updateRequestUser.UserId)
        if err != nil {
            log.Errorf("Error updating business description '%s' for business '%s': %v", businessDescription, businessId, err)
            return model.User{}, model.Business{}, err
        }

        // disable depending features
        if updateRequestUser.ServiceRecommendationEnabled && stringUtil.IsEmptyStringPtr(updateRequestUser.ServiceRecommendation) {
            disableServiceRecommendationEnabledAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "serviceRecommendationEnabled", false)
            if err != nil {
                return model.User{}, model.Business{}, err
            }
            updatedUser, err = userDao.UpdateAttributes(updateRequestUser.UserId, []dbModel.AttributeAction{disableServiceRecommendationEnabledAction})
            if err != nil {
                log.Errorf("Error disabling service recommendation enabled '%s' for user '%s': %v", businessDescription, updateRequestUser.UserId, err)
                return model.User{}, model.Business{}, err
            }
        }
    } else {
        attributeAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "businessDescription", businessDescription)
        if err != nil {
            return model.User{}, model.Business{}, err
        }

        updatedBusiness, err = businessDao.UpdateAttributes(businessId, []dbModel.AttributeAction{attributeAction}, updateRequestUser.UserId)
        if err != nil {
            log.Errorf("Error updating business description '%s' for business '%s': %v", businessDescription, businessId, err)
            return model.User{}, model.Business{}, err
        }
    }

    log.Infof("Successfully processed update business description request for business '%s' requested by user '%s'", businessId, updateRequestUser.UserId)

    return updatedUser, updatedBusiness, nil
}

func handleUpdateSignature(
    user model.User,
    signature string,
    userDao *ddbDao.UserDao,
    log *zap.SugaredLogger) (model.User, error) {
    userId := user.UserId

    var err error
    if stringUtil.IsEmptyString(signature) {
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
    businessId bid.BusinessId,
    updateRequestUserId string,
    keywords string,
    businessDao *ddbDao.BusinessDao,
    log *zap.SugaredLogger) (model.Business, error) {
    var updatedBusiness model.Business
    if stringUtil.IsEmptyString(keywords) {
        removeKeywordsAction, err := dbModel.NewAttributeAction(enum.ActionRemove, "keywords", nil)
        if err != nil {
            log.Errorf("Error creating attribute action: %s", err)
            return model.Business{}, err
        }
        // disable depending features
        disableKeywordEnabledAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "keywordEnabled", false)
        if err != nil {
            log.Errorf("Error creating attribute action: %s", err)
            return model.Business{}, err
        }

        updatedBusiness, err = businessDao.UpdateAttributes(businessId, []dbModel.AttributeAction{removeKeywordsAction, disableKeywordEnabledAction}, updateRequestUserId)
        if err != nil {
            log.Errorf("Error updating keywords '%s' for business '%s': %v", keywords, businessId, err)
            return model.Business{}, err
        }
    } else {
        updateKeywordsAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "keywords", keywords)
        if err != nil {
            log.Errorf("Error creating attribute action: %s", err)
            return model.Business{}, err
        }

        updatedBusiness, err = businessDao.UpdateAttributes(businessId, []dbModel.AttributeAction{updateKeywordsAction}, updateRequestUserId)
        if err != nil {
            log.Errorf("Error updating keywords '%s' for business '%s': %v", keywords, businessId, err)
            return model.Business{}, err
        }
    }

    return updatedBusiness, nil
}

func handleUpdateServiceRecommendation(
    userId string,
    serviceRecommendation string,
    userDao *ddbDao.UserDao,
) (model.User, error) {
    var updatedUser model.User
    if stringUtil.IsEmptyString(serviceRecommendation) {
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
