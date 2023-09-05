package postbackEvent

import (
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
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
)

// handleAutoQuickReplyToggle handles the auto quick reply toggle postback event
// returns autoQuickReplyEnabled, quickReplyMessage ,error
func handleAutoQuickReplyToggle(
    userId string,
    userDao *ddbDao.UserDao,
    businessDao *ddbDao.BusinessDao,
) (bool, *string, error) {
    user, err := userDao.GetUser(userId)
    if err != nil {
        return false, nil, err
    }

    // TODO: [INT-91] Remove backfill logic once all users have been backfilled
    if user.ActiveBusinessId == nil {
        if !*user.AutoQuickReplyEnabled && (util.IsEmptyStringPtr(user.QuickReplyMessage)) {
            return false, nil, exception.NewAutoQuickReplyConditionNotMetException("Please fill in quick reply message before enabling auto quick reply")
        }

        attributeAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "autoQuickReplyEnabled", !*user.AutoQuickReplyEnabled)
        if err != nil {
            return false, nil, err
        }
        updatedUser, err := userDao.UpdateAttributes(user.UserId, []dbModel.AttributeAction{attributeAction})
        if err != nil {
            return false, nil, err
        }
        return *updatedUser.AutoQuickReplyEnabled, updatedUser.QuickReplyMessage, nil
    } else {
        business, err := businessDao.GetBusiness(*user.ActiveBusinessId)
        if err != nil {
            return false, nil, err
        }

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
}

func handleGenerateAiReply(
    event *linebot.Event,
    userId string,
    reviewId _type.ReviewId,
    businessDao *ddbDao.BusinessDao,
    userDao *ddbDao.UserDao,
    reviewDao *ddbDao.ReviewDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) (events.LambdaFunctionURLResponse, error) {
    // get user
    user, err := userDao.GetUser(userId)
    if err != nil {
        log.Errorf("Error getting user '%s' during handling %s: %v", userId, event.Postback.Data, err)

        _, err := line.NotifyUserAiReplyGenerationFailed(userId)
        if err != nil {
            log.Errorf("Error notifying user '%s' that AI reply generation failed: %v", userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error notifying user that AI reply generation failed: %s"}`, err),
            }, err
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Error getting user during handling %s: %s"}`, event.Postback.Data, err),
        }, err
    }

    // get business
    // TODO: [INT-91] Remove backfill logic once all users have been backfilled
    var business *model.Business = nil
    if user.ActiveBusinessId != nil {
        business, err = businessDao.GetBusiness(*user.ActiveBusinessId)
        if err != nil {
            log.Errorf("Error getting business '%s' during handling %s: %v", *user.ActiveBusinessId, event.Postback.Data, err)

            _, err := line.NotifyUserAiReplyGenerationFailed(userId)
            if err != nil {
                log.Errorf("Error notifying user '%s' that AI reply generation failed: %v", userId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error notifying user that AI reply generation failed: %s"}`, err),
                }, err
            }

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error getting business during handling %s: %s"}`, event.Postback.Data, err),
            }, err
        }
    }

    // Notify user that AI is generating reply
    _, err = line.NotifyUserAiReplyGenerationInProgress(event.ReplyToken)
    if err != nil {
        log.Errorf("Error notifying user '%s' that AI is generating reply. Porceeding: %v", userId, err)
    }

    // get review
    review, err := reviewDao.GetReview(userId, reviewId)
    if err != nil {
        log.Errorf("Error getting review during handling %s: %v", event.Postback.Data, err)

        _, err := line.NotifyUserAiReplyGenerationFailed(userId)
        if err != nil {
            log.Errorf("Error notifying user '%s' that AI reply generation failed: %v", userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error notifying user that AI reply generation failed: %s"}`, err),
            }, err
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Error getting review during handling %s: %s"}`, event.Postback.Data, err),
        }, err
    }

    // invoke gpt4
    aiReply, err := aiUtil.NewAi(log).GenerateReply(review.Review, business, user)
    if err != nil {
        log.Errorf("Error generating AI reply: %v", err)

        _, err := line.NotifyUserAiReplyGenerationFailed(userId)
        if err != nil {
            log.Errorf("Error notifying user '%s' that AI reply generation failed: %v", userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error notifying user that AI reply generation failed: %s"}`, err),
            }, err
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Error generating AI reply: %s"}`, err),
        }, err
    }

    // create AI generated result card
    err = line.SendAiGeneratedReply(aiReply, review)
    if err != nil {
        log.Errorf("Error sending AI generated reply to user '%s': %v", userId, err)

        _, err := line.NotifyUserAiReplyGenerationFailed(userId)
        if err != nil {
            log.Errorf("Error notifying user '%s' that AI reply generation failed: %v", userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error notifying user that AI reply generation failed: %s"}`, err),
            }, err
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Error sending AI generated reply: %s"}`, err),
        }, err
    }

    // dummy return
    return events.LambdaFunctionURLResponse{}, nil
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
    userDao *ddbDao.UserDao,
    business *model.Business,
    businessDao *ddbDao.BusinessDao,
    log *zap.SugaredLogger) (*model.Business, model.User, error) {
    // TODO: [INT-91] Remove backfill logic once all users have been backfilled
    if user.ActiveBusinessId == nil {
        if !*user.KeywordEnabled && (util.IsEmptyStringPtr(user.Keywords) || util.IsEmptyStringPtr(user.BusinessDescription)) {
            return nil, model.User{}, exception.NewKeywordConditionNotMetException("Keyword condition not met for " + user.UserId)
        }

        action, err := dbModel.NewAttributeAction(enum.ActionUpdate, "keywordEnabled", !*user.KeywordEnabled)
        if err != nil {
            return nil, model.User{}, err
        }
        updatedUser, err := userDao.UpdateAttributes(user.UserId, []dbModel.AttributeAction{action})
        if err != nil {
            log.Errorf("Error updating keyword enabled to %v for user '%s': %v", !*user.KeywordEnabled, user.UserId, err)
            return nil, model.User{}, err
        }

        return nil, updatedUser, nil
    } else {
        if !business.KeywordEnabled && (util.IsEmptyStringPtr(business.Keywords) || util.IsEmptyStringPtr(business.BusinessDescription)) {
            return nil, model.User{}, exception.NewKeywordConditionNotMetException("Keyword condition not met for " + user.UserId)
        }

        action, err := dbModel.NewAttributeAction(enum.ActionUpdate, "keywordEnabled", !business.KeywordEnabled)
        if err != nil {
            return nil, model.User{}, err
        }
        updatedBusiness, err := businessDao.UpdateAttributes(user.UserId, []dbModel.AttributeAction{action}, user.UserId)
        if err != nil {
            log.Errorf("Error updating keyword enabled to %v for user '%s': %v", !*user.KeywordEnabled, user.UserId, err)
            return nil, model.User{}, err
        }

        return &updatedBusiness, user, nil
    }
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
