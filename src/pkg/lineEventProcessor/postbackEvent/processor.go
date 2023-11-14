package postbackEvent

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/auth"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/dbModel"
    enum3 "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineEventProcessor"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/metric"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/metric/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/rid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
)

func ProcessPostbackEvent(
    event *linebot.Event,
    userId string,
    businessDao *ddbDao.BusinessDao,
    userDao *ddbDao.UserDao,
    reviewDao *ddbDao.ReviewDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) (events.LambdaFunctionURLResponse, error) {

    dataSlice, err := lineEventProcessor.ParsePostBackData(event.Postback.Data)
    if err != nil {
        log.Errorf("Error parsing postback data '%s': %s", event.Postback.Data, err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Error parsing postback data '%s': %s"}`, event.Postback.Data, err),
        }, err
    }

    if len(dataSlice) < 2 {
        return returnUnhandledPostback(log, *event), nil
    }

    // user is empty if event does not require auth
    // WARN: ensure event handlers that require auth are added to shouldAuth() list
    var user model.User
    if shouldAuth(dataSlice) {
        log.Infof("Event requires auth. Validating user '%s' auth...", userId)

        var hasUserCompletedAuth bool
        var err error
        hasUserCompletedAuth, userPtr, err := auth.ValidateUserAuthOrRequestAuth(event.ReplyToken, userId, userDao, line, enum.HandlerNameLineEventsHandler, log)
        if err != nil {
            log.Errorf("Error validating user '%s' auth: %s", userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf("Error validating user '%s' auth: %s", userId, err),
            }, err
        }
        if !hasUserCompletedAuth {
            return events.LambdaFunctionURLResponse{
                StatusCode: 200,
                Body:       `{"message": "User has not completed auth. Prompted auth."}`,
            }, nil
        }
        user = *userPtr

        log.Debugf("Retrieved user: %s", jsonUtil.AnyToJson(user))
    }

    if (dataSlice[0] == "NewReview" || dataSlice[0] == "AiReply") && dataSlice[1] == "GenerateAiReply" {
        // /[NewReview|AiReply]/GenerateAiReply/{BUSINESS_ID}/{REVIEW_ID}
        businessId, err := bid.NewBusinessId(dataSlice[2])
        if err != nil {
            log.Errorf("Error parsing businessId '%s' during handling generate AI reply: %s", dataSlice[2], err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error parsing businessId '%s' during handling generate AI reply: %s"}`, dataSlice[2], err),
            }, err
        }
        reviewId, err := rid.NewReviewId(dataSlice[3])
        if err != nil {
            log.Errorf("Error parsing reviewId '%s' during handling generate AI reply: %s", dataSlice[2], err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error parsing reviewId '%s' during handling generate AI reply: %s"}`, dataSlice[2], err),
            }, err
        }

        err = handleGenerateAiReply(event.ReplyToken, user, businessId, reviewId, businessDao, userDao, reviewDao, line, log)
        if err != nil {
            log.Errorf("Error handling /%s/GenerateAiReply: %s", dataSlice[0], err)

            _, err := line.NotifyUserAiReplyGenerationFailed(userId)
            if err != nil {
                log.Errorf("Error notifying user '%s' that AI reply generation failed: %v", userId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error notifying user that AI reply generation failed: %s"}`, err),
                }, err
            }

            log.Errorf("Successfully notified user '%s' that AI reply generation failed", userId)

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error handling /%s/GenerateAiReply: %s"}`, dataSlice[0], err),
            }, err
        }
    } else {
        switch dataSlice[0] {
        case "NewReview":
            switch dataSlice[1] {
            case "QuickReply":
                log.Info("/NewReview/QuickReply postback event received. User is editing quick reply message before replying")
            case "Reply":
                log.Info("/NewReview/Reply postback event received. User is editing hand-written reply message before replying")
            default:
                return returnUnhandledPostback(log, *event), nil
            }

        case "AiReply":
            switch {
            case bid.IsValidBusinessId(dataSlice[1]):
                businessId := bid.BusinessId(dataSlice[1])
                // user not retrieved if event does not require auth
                if shouldAuth(dataSlice) && !util.StringInSlice(businessId.String(), bid.BusinessIdsToStringSlice(user.BusinessIds)) {
                    log.Errorf("Business ID '%s' does not belong to user '%s'", businessId, userId)
                    return events.LambdaFunctionURLResponse{
                        StatusCode: 500,
                        Body:       fmt.Sprintf(`{"error": "Business ID '%s' does not belong to user '%s'"}`, businessId, userId),
                    }, errors.New("business ID does not belong to user")
                }

                switch dataSlice[2] {
                case "Toggle":
                    // Any toggle will require displaying the AI reply settings, which requires retrieving and/or updating business. Therefore, we will retrieve business here.
                    businessPtr, err := businessDao.GetBusiness(businessId)
                    if err != nil {
                        log.Errorf("Error getting business by businessId '%s'", businessId)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error getting business by businessId '%s'"}`, businessId),
                        }, err
                    }
                    if businessPtr == nil {
                        errStr := fmt.Sprintf("Business not found for businessId: %s", businessId)
                        log.Error(errStr)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Business not found for businessId: %s"}`, businessId),
                        }, errors.New(errStr)
                    }
                    business := *businessPtr

                    switch dataSlice[3] {
                    case "Emoji":
                        user, err = handleEmojiToggle(user, userDao, log)
                        if err != nil {
                            log.Errorf("Error handling emoji toggle: %s", err)

                            _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "Emoji AI 回覆")
                            if err != nil {
                                log.Errorf("Error notifying user '%s' of update emoji enabled failed: %v", userId, err)
                                metric.EmitLambdaMetric(enum2.Metric5xxError, enum.HandlerNameLineEventsHandler, 1)
                            }

                            return events.LambdaFunctionURLResponse{
                                StatusCode: 500,
                                Body:       fmt.Sprintf(`{"error": "Error handling emoji toggle: %s"}`, err),
                            }, nil
                        }

                    case "Signature":
                        user, err = handleSignatureToggle(user, userDao, log)

                        if err != nil {
                            var signatureDoesNotExistException *exception.SignatureDoesNotExistException
                            if errors.As(err, &signatureDoesNotExistException) {
                                _, err = line.ReplyUser(event.ReplyToken, "請先填寫簽名，才能開啟簽名功能")
                                if err != nil {
                                    log.Errorf("Error replying signature settings prompt message to user '%s': %v", userId, err)
                                    return events.LambdaFunctionURLResponse{
                                        StatusCode: 200,
                                        Body:       fmt.Sprintf(`{"error": "Error replying signature settings prompt message: %s"}`, err),
                                    }, err
                                }
                            }

                            log.Errorf("Error handling signature toggle: %s", err)
                            return events.LambdaFunctionURLResponse{
                                StatusCode: 500,
                                Body:       fmt.Sprintf(`{"error": "Error handling signature toggle: %s"}`, err),
                            }, err
                        }

                    case "Keyword":
                        business, err = handleKeywordToggle(user, business, businessDao)
                        if err != nil {
                            log.Errorf("Error handling keyword toggle: %s", err)

                            var keywordConditionNotMetException *exception.KeywordConditionNotMetException
                            if errors.As(err, &keywordConditionNotMetException) {
                                _, err = line.ReplyUser(event.ReplyToken, "請先填寫主要業務及關鍵字，才能開啟關鍵字回覆功能")
                                if err != nil {
                                    log.Errorf("Error replying keyword settings prompt message to user '%s': %v", userId, err)
                                    return events.LambdaFunctionURLResponse{
                                        StatusCode: 500,
                                        Body:       fmt.Sprintf(`{"error": "Error replying keyword settings prompt message: %s"}`, err),
                                    }, err
                                }
                            }

                            return events.LambdaFunctionURLResponse{
                                StatusCode: 500,
                                Body:       fmt.Sprintf(`{"error": "Error handling keyword toggle: %s"}`, err),
                            }, err
                        }

                    case "ServiceRecommendation":
                        user, err = handleServiceRecommendationToggle(user, business.BusinessDescription, userDao, log)
                        if err != nil {
                            var serviceRecommendationConditionNotMetException *exception.ServiceRecommendationConditionNotMetException
                            if errors.As(err, &serviceRecommendationConditionNotMetException) {
                                _, err := line.ReplyUser(event.ReplyToken, "請先填寫推薦業務或主要業務欄位，才能開啟推薦其他業務功能")
                                if err != nil {
                                    log.Errorf("Error replying service recommendation settings prompt message to user '%s': %v", userId, err)
                                    return events.LambdaFunctionURLResponse{
                                        StatusCode: 500,
                                        Body:       fmt.Sprintf(`{"error": "Error replying service recommendation settings prompt message: %s"}`, err),
                                    }, err
                                }

                                return events.LambdaFunctionURLResponse{
                                    StatusCode: 200,
                                    Body:       fmt.Sprintf(`{"Rejected enabling service recommendation feature": "Please fill in service recommendation before enabling service recommendation"}`),
                                }, nil
                            }

                            log.Errorf("Error handling service recommendation toggle: %s", err)
                            return events.LambdaFunctionURLResponse{
                                StatusCode: 500,
                                Body:       fmt.Sprintf(`{"error": "Error handling keyword toggle: %s"}`, err),
                            }, err
                        }
                    }

                    // notify all other users of toggle (skip notifying self)
                    err = line.NotifyAiReplySettingsUpdated(util.RemoveStringFromSlice(business.UserIds, userId), user.LineUsername, business.BusinessName)
                    if err != nil {
                        log.Errorf("Error notifying other users of AI reply settings update for user '%s': %v", userId, err)
                    }

                    err = line.ShowAiReplySettings(event.ReplyToken, user, business, businessDao)
                    if err != nil {
                        log.Errorf("Error sending AI reply settings to user '%s': %v", userId, err)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error sending AI reply settings: %s"}`, err),
                        }, err
                    }

                case "EditBusinessDescription":
                    log.Info("/AiReply/EditBusinessDescription postback event received. User is editing business description for ", businessId)

                case "EditSignature":
                    log.Info("/AiReply/EditSignature postback event received. User is editing signature")

                case "EditKeywords":
                    log.Info("/AiReply/EditKeywords postback event received. User is editing keywords")

                case "EditServiceRecommendations":
                    log.Info("/AiReply/EditServiceRecommendations postback event received. User is editing service recommendations for ", businessId)

                case "EditReply":
                    log.Info("/AiReply/EditReply postback event received. User is editing AI generated reply for ", businessId)

                case "UpdateActiveBusiness":
                    // /AiReply/{BUSINESS_ID}/UpdateActiveBusiness
                    // update active business ID for user
                    action, err := dbModel.NewAttributeAction(enum3.ActionUpdate, "activeBusinessId", businessId.String())
                    if err != nil {
                        log.Errorf("Error creating attribute action: %s", err)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error creating attribute action: %s"}`, err),
                        }, err
                    }

                    user, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{action})
                    if err != nil {
                        log.Errorf("Error updating user '%s' active business ID to '%s': %s", userId, businessId, err)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error updating user active business ID: %s"}`, err),
                        }, err
                    }

                    err = line.ShowAiReplySettingsByUser(event.ReplyToken, user, businessDao)
                    if err != nil {
                        log.Errorf("Error sending AI reply settings to user '%s': %v", userId, err)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error sending AI reply settings: %s"}`, err),
                        }, err
                    }

                default:
                    return returnUnhandledPostback(log, *event), nil
                }
            default:
                return returnUnhandledPostback(log, *event), nil
            }

        case "RichMenu":
            switch dataSlice[1] {
            case "QuickReplySettings":
                err = line.ShowQuickReplySettings(
                    event.ReplyToken, user, businessDao)
                if err != nil {
                    log.Errorf("Error sending quick reply settings to user '%s': %v", userId, err)
                    return events.LambdaFunctionURLResponse{
                        StatusCode: 500,
                        Body:       fmt.Sprintf(`{"error": "Error sending quick reply settings: %s"}`, err),
                    }, err
                }

            case "AiReplySettings":
                err = line.ShowAiReplySettingsByUser(event.ReplyToken, user, businessDao)
                if err != nil {
                    log.Errorf("Error sending AI reply settings to user '%s': %v", userId, err)
                    return events.LambdaFunctionURLResponse{
                        StatusCode: 500,
                        Body:       fmt.Sprintf(`{"error": "Error sending AI reply settings: %s"}`, err),
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
            //
            // case "More":
            //     _, err := line.ReplyMoreMessage(event.ReplyToken)
            //     if err != nil {
            //         log.Errorf("Error replying More message to user '%s': %v", userId, err)
            //         return events.LambdaFunctionURLResponse{
            //             StatusCode: 500,
            //             Body:       fmt.Sprintf(`{"error": "Error replying more message: %s"}`, err),
            //         }, err
            //     }

            default:
                return returnUnhandledPostback(log, *event), nil
            }

        case "QuickReply":
            switch {
            case bid.IsValidBusinessId(dataSlice[1]):
                businessId := bid.BusinessId(dataSlice[1])

                // validate businessId belongs to user
                if shouldAuth(dataSlice) && !util.StringInSlice(businessId.String(), bid.BusinessIdsToStringSlice(user.BusinessIds)) {
                    log.Errorf("Business ID '%s' does not belong to user '%s'", businessId, userId)
                    log.Debugf("User's Business IDs: %s", jsonUtil.AnyToJson(user.BusinessIds))

                    return events.LambdaFunctionURLResponse{
                        StatusCode: 500,
                        Body:       fmt.Sprintf(`{"error": "Business ID '%s' does not belong to user '%s'"}`, businessId, userId),
                    }, errors.New("business ID does not belong to user")
                }

                switch dataSlice[2] {
                case "Toggle":
                    switch dataSlice[3] {
                    case "AutoReply":
                        business, err := handleAutoQuickReplyToggle(user, businessId, businessDao, log)
                        if err != nil {
                            var autoQuickReplyConditionNotMetException *exception.AutoQuickReplyConditionNotMetException
                            if errors.As(err, &autoQuickReplyConditionNotMetException) {
                                log.Warnf("Auto reply condition not met for user '%s': %v", userId, err)
                                _, replyUserErr := line.ReplyUser(event.ReplyToken, "請先填寫快速回覆訊息，才能開啟自動回覆功能")
                                if replyUserErr != nil {
                                    log.Errorf("Error replying cannot enable auto quick reply prompt to user '%s': %v", userId, replyUserErr)

                                    return events.LambdaFunctionURLResponse{
                                        StatusCode: 500,
                                        Body:       fmt.Sprintf(`{"error": "Error replying cannot enable auto quick reply prompt: %s"}`, replyUserErr),
                                    }, replyUserErr
                                }
                                log.Warnf("Notified user '%s' to fill in quick reply message before enabling auto quick reply", userId)

                                return events.LambdaFunctionURLResponse{
                                    StatusCode: 200,
                                    Body:       fmt.Sprintf(`{"Rejected enabling auto quick reply feature": "Please fill in quick reply message before enabling auto quick reply"}`),
                                }, nil
                            }

                            log.Errorf("Error handling auto quick reply toggle for user '%s': %v", userId, err)

                            _, notifyUserErr := line.NotifyUserUpdateFailed(event.ReplyToken, "自動回覆")
                            if notifyUserErr != nil {
                                log.Errorf("Error notifying user of updating auto quick reply enabled failed for user '%s': %v", userId, notifyUserErr)
                                metric.EmitLambdaMetric(enum2.Metric5xxError, enum.HandlerNameLineEventsHandler, 1)
                            }

                            return events.LambdaFunctionURLResponse{
                                StatusCode: 500,
                                Body:       fmt.Sprintf(`{"error": "Error handling auto quick reply toggle: %s"}`, notifyUserErr),
                            }, err
                        }

                        // notify all other users of toggle (skip notifying self)
                        err = line.NotifyQuickReplySettingsUpdated(util.RemoveStringFromSlice(business.UserIds, userId), user.LineUsername, business.BusinessName)
                        if err != nil {
                            log.Errorf("Error notifying other users of quick reply settings update for user '%s': %v", userId, err)
                        }

                        err = line.ShowQuickReplySettingsWithActiveBusiness(event.ReplyToken, user, business, businessDao)
                        if err != nil {
                            log.Errorf("Error showing quick reply settings for user '%s': %v", userId, err)
                            return events.LambdaFunctionURLResponse{
                                StatusCode: 500,
                                Body:       fmt.Sprintf(`{"error": "Failed to show quick reply settings: %s"}`, err),
                            }, err
                        }
                    default:
                        return returnUnhandledPostback(log, *event), nil
                    }

                case "EditQuickReplyMessage":
                    // /QuickReply/{BUSINESS_ID}/EditQuickReplyMessage
                    log.Info("/QuickReply/EditQuickReplyMessage postback event received. User is editing quick reply message.")

                case "UpdateActiveBusiness":
                    // /QuickReply/{BUSINESS_ID}/UpdateActiveBusiness
                    // update active business ID for user
                    action, err := dbModel.NewAttributeAction(enum3.ActionUpdate, "activeBusinessId", businessId.String())
                    if err != nil {
                        log.Errorf("Error creating attribute action: %s", err)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error creating attribute action: %s"}`, err),
                        }, err
                    }

                    user, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{action})
                    if err != nil {
                        log.Errorf("Error updating user '%s' active business ID to '%s': %s", userId, businessId, err)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error updating user active business ID: %s"}`, err),
                        }, err
                    }

                    err = line.ShowQuickReplySettings(event.ReplyToken, user, businessDao)
                    if err != nil {
                        log.Errorf("Error sending AI reply settings to user '%s': %v", userId, err)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 500,
                            Body:       fmt.Sprintf(`{"error": "Error sending AI reply settings: %s"}`, err),
                        }, err
                    }

                default:
                    return returnUnhandledPostback(log, *event), nil
                }

            default:
                return returnUnhandledPostback(log, *event), nil
            }

        case "Notification":
            switch dataSlice[1] {

            case "Replied":
                switch dataSlice[2] {
                case "Reply":
                    // /Notification/Replied/Reply
                    log.Info("/Notification/Replied/Reply postback event received. User is editing reply message to be resent.")
                default:
                    return returnUnhandledPostback(log, *event), nil
                }

            default:
                return returnUnhandledPostback(log, *event), nil
            }

        default:
            return returnUnhandledPostback(log, *event), nil
        }
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

func shouldAuth(postbackEvent []string) bool {
    return !(postbackEvent[0] == "RichMenu" && postbackEvent[1] == "Help") &&
        !(postbackEvent[0] == "QuickReply" && postbackEvent[2] == "EditQuickReplyMessage") &&
        !(postbackEvent[0] == "AiReply" && postbackEvent[2] == "EditBusinessDescription") &&
        !(postbackEvent[0] == "AiReply" && postbackEvent[2] == "EditSignature") &&
        !(postbackEvent[0] == "AiReply" && postbackEvent[2] == "EditKeywords") &&
        !(postbackEvent[0] == "AiReply" && postbackEvent[2] == "EditServiceRecommendations") &&
        !(postbackEvent[0] == "Notification" && postbackEvent[1] == "Replied" && postbackEvent[2] == "Reply")
}
