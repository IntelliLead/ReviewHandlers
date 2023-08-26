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

func HandleBusinessDescriptionUpdate(
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
            *updatedBusiness, err = businessDao.UpdateAttributes(business.BusinessId, attributeActions)
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
            *updatedBusiness, err = businessDao.UpdateAttributes(business.BusinessId, []dbModel.AttributeAction{attributeAction})
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
