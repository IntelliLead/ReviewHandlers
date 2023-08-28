package messageEvent

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/dbModel"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
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
    userId string,
    quickReplyMessage string,
    businessDao *ddbDao.BusinessDao,
    userDao *ddbDao.UserDao,
    log *zap.SugaredLogger) (bool, *string, error) {
    // TODO: [INT-91] Remove backfill logic once all users have been backfilled
    user, err := userDao.GetUser(userId)
    if err != nil {
        return false, nil, err
    }

    actions, err := buildQuickReplyUpdateAttributeActions(quickReplyMessage)
    if err != nil {
        return false, nil, err
    }
    if user.ActiveBusinessId == nil {
        updatedUser, err := userDao.UpdateAttributes(userId, actions)
        if err != nil {
            log.Errorf("Error updating quick reply message '%s' for user '%s': %v", quickReplyMessage, userId, err)
            return false, nil, err
        }

        return *updatedUser.AutoQuickReplyEnabled, updatedUser.QuickReplyMessage, nil
    } else {
        updatedBusiness, err := businessDao.UpdateAttributes(*user.ActiveBusinessId, actions, userId)
        if err != nil {
            log.Errorf("Error updating quick reply message '%s' for business '%s': %v", quickReplyMessage, *user.ActiveBusinessId, err)
            return false, nil, err
        }

        return updatedBusiness.AutoQuickReplyEnabled, updatedBusiness.QuickReplyMessage, nil
    }
}

func handleBusinessDescriptionUpdate(
    userId string,
    replyToken string,
    businessDescription string,
    userDao *ddbDao.UserDao,
    businessDao *ddbDao.BusinessDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) error {
    user, err := userDao.GetUser(userId)
    if err != nil {
        log.Errorf("Error getting user '%s': %v", userId, err)
        return err
    }

    // TODO: [INT-91] Remove backfill logic once all users have been backfilled
    isBackfilled := true
    var business *model.Business
    // handle un-backfilled user with legacy logic
    if user.ActiveBusinessId == nil {
        isBackfilled = false
    } else {
        business, err = businessDao.GetBusiness(*user.ActiveBusinessId)
        if err != nil {
            log.Errorf("Error getting business '%s': %v", *user.ActiveBusinessId, err)
            return err
        }
    }

    // update DDB
    var updatedUser model.User
    var updatedBusiness *model.Business
    if util.IsEmptyString(businessDescription) {
        removeBusinessDescriptionAction, err := dbModel.NewAttributeAction(enum.ActionRemove, "businessDescription", nil)
        if err != nil {
            return err
        }
        disableKeywordEnabledAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "keywordEnabled", false)
        if err != nil {
            return err
        }

        attributeActions := []dbModel.AttributeAction{
            removeBusinessDescriptionAction,
            // disable depending features
            disableKeywordEnabledAction,
        }

        // disable depending features
        var disableServiceRecommendationEnabledAction *dbModel.AttributeAction
        if !util.IsEmptyStringPtr(user.ServiceRecommendation) {
            *disableServiceRecommendationEnabledAction, err = dbModel.NewAttributeAction(enum.ActionUpdate, "serviceRecommendationEnabled", false)
            if err != nil {
                return err
            }
        }

        if isBackfilled {
            *updatedBusiness, err = businessDao.UpdateAttributes(business.BusinessId, attributeActions, userId)
            if err != nil {
                log.Errorf("Error updating business description '%s' for business '%s': %v", businessDescription, business.BusinessId, err)
                return err
            }

            if disableServiceRecommendationEnabledAction != nil {
                updatedUser, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{*disableServiceRecommendationEnabledAction})
                if err != nil {
                    log.Errorf("Error disabling service recommendation enabled '%s' for user '%s': %v", businessDescription, userId, err)
                    return err
                }
            }

        } else {
            if disableServiceRecommendationEnabledAction != nil {
                attributeActions = append(attributeActions, dbModel.AttributeAction{Action: enum.ActionUpdate, Name: "serviceRecommendationEnabled", Value: false})
            }
            updatedUser, err = userDao.UpdateAttributes(userId, attributeActions)
            if err != nil {
                log.Errorf("Error updating business description '%s' for user '%s': %v", businessDescription, userId, err)
                return err
            }
        }
    } else {
        attributeAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "businessDescription", businessDescription)
        if err != nil {
            return err
        }

        if isBackfilled {
            *updatedBusiness, err = businessDao.UpdateAttributes(business.BusinessId, []dbModel.AttributeAction{attributeAction}, userId)
            if err != nil {
                log.Errorf("Error updating business description '%s' for business '%s': %v", businessDescription, business.BusinessId, err)
                return err
            }
        } else {
            updatedUser, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{attributeAction})
        }
    }
    if err != nil {
        log.Errorf("Error updating business description '%s' for user '%s': %v", businessDescription, userId, err)

        _, err := line.NotifyUserUpdateFailed(replyToken, "主要業務")
        if err != nil {
            return nil
        }
        log.Error("Successfully notified user of update business description failed")

        return nil
    }

    err = line.ShowAiReplySettings(replyToken, updatedUser, updatedBusiness)
    if err != nil {
        log.Errorf("Error showing seo settings for user '%s': %v", userId, err)
        return nil
    }

    log.Infof("Successfully processed update business description request for user '%s'", userId)

    return nil
}

func handleUpdateSignature(
    userId string,
    signature string,
    userDao *ddbDao.UserDao,
    businessDao *ddbDao.BusinessDao,
    log *zap.SugaredLogger) (*model.User, *model.Business, error) {
    var updatedUser model.User
    var err error
    if util.IsEmptyString(signature) {
        updatedUser, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{
            {Action: enum.ActionRemove, Name: "signature"},
            // disable depending features
            {Action: enum.ActionUpdate, Name: "signatureEnabled", Value: false},
        })
    } else {
        updatedUser, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{
            {Action: enum.ActionUpdate, Name: "signature", Value: signature},
        })
    }
    if err != nil {
        log.Errorf("Error updating signature '%s' for user '%s': %v", signature, userId, err)
        return nil, nil, err
    }

    // TODO: [INT-91] Remove backfill logic once all users have been backfilled
    var business *model.Business = nil
    if updatedUser.ActiveBusinessId != nil {
        business, err = businessDao.GetBusiness(*updatedUser.ActiveBusinessId)
        if err != nil {
            log.Errorf("Error getting business '%s': %v", *updatedUser.ActiveBusinessId, err)
            return nil, nil, err
        }
    }

    return &updatedUser, business, nil
}

func handleUpdateKeywords(
    userId string,
    keywords string,
    userDao *ddbDao.UserDao,
    businessDao *ddbDao.BusinessDao,
    log *zap.SugaredLogger) (*model.User, *model.Business, error) {
    // TODO: [INT-91] Remove backfill logic once all users have been backfilled
    user, err := userDao.GetUser(userId)
    if err != nil {
        log.Errorf("Error getting user '%s': %v", userId, err)
        return nil, nil, err
    }
    if user.ActiveBusinessId == nil {
        var updatedUser model.User
        var err error
        if util.IsEmptyString(keywords) {
            updatedUser, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{
                {Action: enum.ActionRemove, Name: "keywords"},
                // disable depending features
                {Action: enum.ActionUpdate, Name: "keywordEnabled", Value: false},
            })
        } else {
            updatedUser, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{
                {Action: enum.ActionUpdate, Name: "keywords", Value: keywords},
            })
        }
        if err != nil {
            log.Errorf("Error updating keywords '%s' for unbackfilled user '%s': %v", keywords, userId, err)
            return &updatedUser, nil, err
        }

        return &updatedUser, nil, nil
    } else {

        var updatedBusiness *model.Business
        if util.IsEmptyString(keywords) {
            removeKeywordsAction, err := dbModel.NewAttributeAction(enum.ActionRemove, "keywords", nil)
            if err != nil {
                return nil, nil, err
            }
            // disable depending features
            disableKeywordEnabledAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "keywordEnabled", false)
            if err != nil {
                return nil, nil, err
            }

            *updatedBusiness, err = businessDao.UpdateAttributes(*user.ActiveBusinessId, []dbModel.AttributeAction{removeKeywordsAction, disableKeywordEnabledAction}, userId)
        } else {
            updateKeywordsAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "keywords", keywords)
            if err != nil {
                return nil, nil, err
            }

            *updatedBusiness, err = businessDao.UpdateAttributes(*user.ActiveBusinessId, []dbModel.AttributeAction{updateKeywordsAction}, userId)
        }
        if err != nil {
            log.Errorf("Error updating keywords '%s' for backfilled user '%s' with business '%s': %v", keywords, userId, *user.ActiveBusinessId, err)
            return nil, updatedBusiness, err
        }

        return &user, updatedBusiness, nil
    }
}
