package lineEventProcessor

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/aiUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/dbModel"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
    "strings"
)

func ProcessPostbackEvent(event *linebot.Event,
    userId string,
    businessDao *ddbDao.BusinessDao,
    userDao *ddbDao.UserDao,
    reviewDao *ddbDao.ReviewDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) (events.LambdaFunctionURLResponse, error) {

    // parse data - expect path format: e.g., /RichMenu/QuickReplySettings
    dataSlice := strings.Split(event.Postback.Data, "/")
    if len(dataSlice) < 2 {
        return returnUnhandledPostback(log, *event), nil
    }
    // shift off the first element, which is empty
    dataSlice = dataSlice[1:]

    switch dataSlice[0] {
    // /[NewReview|AiReply]/GenerateAiReply/{REVIEW_ID}
    case "NewReview", "AiReply":
        if len(dataSlice) == 3 && dataSlice[1] == "GenerateAiReply" && !util.IsEmptyString(dataSlice[2]) {
            reviewId := _type.NewReviewId(dataSlice[2])
            lambdaReturn, err := handleGenerateAiReply(event, userId, reviewId, userDao, reviewDao, line, log)
            if err != nil {
                return lambdaReturn, err
            } // else continue

        } else if dataSlice[0] == "NewReview" {
            if dataSlice[1] == "QuickReply" {
                log.Info("/NewReview/QuickReply postback event received. User is editing quick reply message before replying")
            }
            if dataSlice[1] == "Reply" {
                log.Info("/NewReview/Reply postback event received. User is editing hand-written reply message before replying")
            }

        } else if dataSlice[0] == "AiReply" && len(dataSlice) >= 2 {
            switch dataSlice[1] {
            case "Toggle":
                if len(dataSlice) != 3 || util.IsEmptyString(dataSlice[2]) {
                    return returnUnhandledPostback(log, *event), nil
                }

                // get user
                user, err := userDao.GetUser(userId)
                if err != nil {
                    log.Error("Error getting user during handling /AiReply/KeywordToggle: ", err)
                    return events.LambdaFunctionURLResponse{
                        StatusCode: 500,
                        Body:       fmt.Sprintf(`{"error": "Error getting user: %s"}`, err),
                    }, err
                }

                // TODO: [INT-91] Remove backfill logic once all users have been backfilled
                var business *model.Business
                if user.ActiveBusinessId != nil {
                    business, err = businessDao.GetBusiness(*user.ActiveBusinessId)
                    if err != nil {
                        log.Errorf("Error getting business %s: %s", *user.ActiveBusinessId, err)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error getting business: %s"}`, err),
                        }, err
                    }
                }

                var updatedUser model.User
                var updatedBusiness *model.Business
                switch dataSlice[2] {
                case "Emoji":
                    updatedUser, err = handleEmojiToggle(user, userDao, log)
                    if err != nil {
                        log.Errorf("Error handling emoji toggle: %s", err)

                        // notify user of error
                        _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "Emoji AI 回覆")
                        if err != nil {
                            log.Errorf("Error notifying user '%s' of update emoji enabled failed: %v", user.UserId, err)
                        }

                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error handling emoji toggle: %s"}`, err),
                        }, nil
                    }

                case "Signature":
                    updatedUser, err = handleSignatureToggle(user, userDao, log)

                    if err != nil {
                        log.Errorf("Error handling signature toggle: %s", err)
                        if _, ok := err.(*exception.SignatureDoesNotExistException); ok {
                            _, err := line.ReplyUser(event.ReplyToken, "請先填寫簽名，才能開啟簽名功能")
                            if err != nil {
                                log.Errorf("Error replying signature settings prompt message to user '%s': %v", user.UserId, err)
                                return events.LambdaFunctionURLResponse{
                                    StatusCode: 500,
                                    Body:       fmt.Sprintf(`{"error": "Error replying signature settings prompt message: %s"}`, err),
                                }, err
                            }
                        } else {
                            _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "簽名 AI 回覆")
                            if err != nil {
                                return events.LambdaFunctionURLResponse{
                                    StatusCode: 500,
                                    Body:       fmt.Sprintf(`{"error": "Failed to notify user of update signature enabled failed: %s"}`, err),
                                }, err
                            }
                        }

                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error handling signature toggle: %s"}`, err),
                        }, err
                    }

                case "Keyword":
                    updatedBusiness, updatedUser, err = handleKeywordToggle(user, userDao, business, businessDao, log)
                    if err != nil {
                        log.Errorf("Error handling keyword toggle: %s", err)

                        if _, ok := err.(*exception.KeywordConditionNotMetException); ok {
                            _, err := line.ReplyUser(event.ReplyToken, "請先填寫主要業務及關鍵字，才能開啟關鍵字回覆功能")
                            if err != nil {
                                log.Errorf("Error replying keyword settings prompt message to user '%s': %v", user.UserId, err)
                                return events.LambdaFunctionURLResponse{
                                    StatusCode: 500,
                                    Body:       fmt.Sprintf(`{"error": "Error replying keyword settings prompt message: %s"}`, err),
                                }, err
                            }
                        } else {
                            _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "關鍵字 AI 回覆")
                            if err != nil {
                                log.Errorf("Error notifying user '%s' of update keyword enabled failed: %v", user.UserId, err)
                                return events.LambdaFunctionURLResponse{
                                    StatusCode: 500,
                                    Body:       fmt.Sprintf(`{"error": "Failed to notify user of update keyword enabled failed: %s"}`, err),
                                }, err
                            }
                        }

                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error handling keyword toggle: %s"}`, err),
                        }, err
                    }

                case "ServiceRecommendation":
                    updatedUser, err = handleServiceRecommendationToggle(user, userDao, log)
                    if err != nil {
                        log.Errorf("Error handling service recommendation toggle: %s", err)

                        if _, ok := err.(*exception.ServiceRecommendationConditionNotMetException); ok {
                            _, err := line.ReplyUser(event.ReplyToken, "請先填寫推薦業務或主要業務欄位，才能開啟推薦其他業務功能")
                            if err != nil {
                                log.Errorf("Error replying service recommendation settings prompt message to user '%s': %v", user.UserId, err)
                                return events.LambdaFunctionURLResponse{
                                    StatusCode: 500,
                                    Body:       fmt.Sprintf(`{"error": "Error replying service recommendation settings prompt message: %s"}`, err),
                                }, err
                            }
                        } else {
                            _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "推薦其他業務 AI 回覆")
                            if err != nil {
                                log.Errorf("Error notifying user '%s' of update service recommendation enabled failed: %v", user.UserId, err)
                                return events.LambdaFunctionURLResponse{
                                    StatusCode: 500,
                                    Body:       fmt.Sprintf(`{"error": "Failed to notify user of update service recommendation enabled failed: %s"}`, err),
                                }, err
                            }
                        }

                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error handling keyword toggle: %s"}`, err),
                        }, err
                    }

                }

                err = line.ShowAiReplySettings(event.ReplyToken, updatedUser, updatedBusiness)
                if err != nil {
                    log.Errorf("Error showing updated AI Reply settings for user '%s': %v", user.UserId, err)
                    return events.LambdaFunctionURLResponse{
                        StatusCode: 500,
                        Body:       fmt.Sprintf(`{"error": "Failed to show updated AI Reply settings: %s"}`, err),
                    }, err
                }

            case "EditBusinessDescription":
                log.Info("/AiReply/EditBusinessDescription postback event received. User is editing business description")

            case "EditSignature":
                log.Info("/AiReply/EditSignature postback event received. User is editing signature")

            case "EditKeywords":
                log.Info("/AiReply/EditKeywords postback event received. User is editing keywords")

            case "EditServiceRecommendations":
                log.Info("/AiReply/EditServiceRecommendations postback event received. User is editing service recommendations")
            }

        } else {
            return returnUnhandledPostback(log, *event), nil
        }

    case "RichMenu":
        if len(dataSlice) < 2 {
            return returnUnhandledPostback(log, *event), nil
        }

        switch dataSlice[1] {
        case "QuickReplySettings":
            // get user
            user, err := userDao.GetUser(userId)
            if err != nil {
                log.Error("Error getting user during handling /RichMenu/QuickReplySettings: ", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error getting user: %s"}`, err),
                }, err
            }

            err = line.ShowQuickReplySettings(event.ReplyToken, user)
            if err != nil {
                log.Errorf("Error sending quick reply settings to user '%s': %v", user.UserId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error sending quick reply settings: %s"}`, err),
                }, err
            }

        case "AiReplySettings":
            // get user
            user, err := userDao.GetUser(userId)
            if err != nil {
                log.Error("Error getting user during handling /RichMenu/AiReplySettings: ", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error getting user: %s"}`, err),
                }, err
            }
            // TODO: [INT-91] Remove backfill logic once all users have been backfilled
            var business *model.Business
            if user.ActiveBusinessId != nil {
                business, err = businessDao.GetBusiness(*user.ActiveBusinessId)
                if err != nil {
                    log.Error("Error getting business during handling /RichMenu/AiReplySettings: ", err)
                    return events.LambdaFunctionURLResponse{
                        StatusCode: 500,
                        Body:       fmt.Sprintf(`{"error": "Error getting business: %s"}`, err),
                    }, err
                }
            }

            err = line.ShowAiReplySettings(event.ReplyToken, user, business)
            if err != nil {
                log.Errorf("Error sending seo settings to user '%s': %v", user.UserId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error sending seo settings: %s"}`, err),
                }, err
            }

        case "Help":
            _, err := line.ReplyHelpMessage(event.ReplyToken)
            if err != nil {
                log.Errorf("Error replying help message to user '%s': %v", userId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error replying help message: %s"}`, err),
                }, err
            }

        case "More":
            _, err := line.ReplyMoreMessage(event.ReplyToken)
            if err != nil {
                log.Errorf("Error replying More message to user '%s': %v", userId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error replying more message: %s"}`, err),
                }, err
            }

        default:
            log.Error("Unknown RichMenu postback data: ", dataSlice[1])
            return events.LambdaFunctionURLResponse{
                StatusCode: 400,
                Body:       fmt.Sprintf(`{"message": "Unknown RichMenu postback data: %s"}`, dataSlice[1]),
            }, nil
        }

    case "QuickReply":
        if len(dataSlice) < 2 {
            return returnUnhandledPostback(log, *event), nil
        }

        switch dataSlice[1] {
        case "Toggle":
            // /QuickReply/Toggle/AutoReply
            if len(dataSlice) != 3 || util.IsEmptyString(dataSlice[2]) || dataSlice[2] != "AutoReply" {
                return returnUnhandledPostback(log, *event), nil
            }

            // get user
            user, err := userDao.GetUser(userId)
            if err != nil {
                log.Error("Error getting user during handling /QuickReply/Toggle/AutoReply: ", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error getting user: %s"}`, err),
                }, err
            }

            lambdaReturn, err := handleAutoQuickReplyToggle(event.ReplyToken, user, userDao, line, log)
            if err != nil {
                return lambdaReturn, err
            } // else continue

        case "EditQuickReplyMessage":
            // /QuickReply/EditQuickReplyMessage
            log.Info("/QuickReply/EditQuickReplyMessage postback event received. User is editing quick reply message")
        }

    default:
        log.Warn("Unknown QuickReply postback data: ", dataSlice)
        return events.LambdaFunctionURLResponse{
            StatusCode: 400,
            Body:       "{\"message\": \"Unknown QuickReply postback data\"}",
        }, nil
    }

    log.Infof("Successfully handled Postback event from user '%s': %s", userId, jsonUtil.AnyToJson(event.Postback.Data))

    return events.LambdaFunctionURLResponse{Body: `{"message": "Successfully handled Postback event"}`, StatusCode: 200}, nil
}

func returnUnhandledPostback(log *zap.SugaredLogger, event linebot.Event) events.LambdaFunctionURLResponse {
    log.Error("Postback event data is not in expected format. No action taken: ", event.Postback.Data)
    return events.LambdaFunctionURLResponse{
        StatusCode: 200,
        Body:       `{"message": "Postback event data is not in expected format. No action taken."}`,
    }
}

func handleGenerateAiReply(event *linebot.Event,
    userId string,
    reviewId _type.ReviewId,
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
    aiReply, err := aiUtil.NewAi(log).GenerateReply(review.Review, user)
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
        updatedBusiness, err := businessDao.UpdateAttributes(user.UserId, []dbModel.AttributeAction{action})
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

func handleAutoQuickReplyToggle(replyToken string,
    user model.User,
    userDao *ddbDao.UserDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) (events.LambdaFunctionURLResponse, error) {

    if !user.AutoQuickReplyEnabled && (util.IsEmptyStringPtr(user.QuickReplyMessage)) {
        _, err := line.ReplyUser(replyToken, "請先填寫快速回覆訊息，才能開啟自動回覆功能")
        if err != nil {
            log.Errorf("Error replying cannot enable auto quick reply prompt to user '%s': %v", user.UserId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 00,
                Body:       fmt.Sprintf(`{"error": "Error replying cannot enable auto quick reply prompt: %s"}`, err),
            }, err
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       fmt.Sprintf(`{"Rejected enabling auto quick reply feature": "Please fill in quick reply message before enabling auto quick reply"}`),
        }, nil
    }

    attributeAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "autoQuickReplyEnabled", !user.AutoQuickReplyEnabled)
    if err != nil {
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Error creating attribute action: %s"}`, err),
        }, err
    }
    updatedUser, err := userDao.UpdateAttributes(user.UserId, []dbModel.AttributeAction{attributeAction})
    if err != nil {
        log.Errorf("Error updating auto quick reply enabled to %v for user '%s': %v", !user.AutoQuickReplyEnabled, user.UserId, err)

        // notify user of error
        _, err := line.NotifyUserUpdateFailed(replyToken, "自動回覆")
        if err != nil {
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to notify user of updating auto quick reply enabled failed: %s"}`, err),
            }, err
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Error updating auto quick reply enabled: %s"}`, err),
        }, err
    }

    err = line.ShowQuickReplySettings(replyToken, updatedUser)
    if err != nil {
        log.Errorf("Error showing quick reply settings for user '%s': %v", user.UserId, err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to show quick reply settings: %s"}`, err),
        }, err
    }

    // dummy return
    return events.LambdaFunctionURLResponse{
        StatusCode: 200,
        Body:       fmt.Sprintf(`{"Success": "Successfully updated quick reply enabled"}`),
    }, nil
}
