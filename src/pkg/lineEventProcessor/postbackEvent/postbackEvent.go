package postbackEvent

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/auth"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
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

func shouldAuth(postbackEvent []string) bool {
    return postbackEvent[1] == "GenerateAiReply" || postbackEvent[1] == "Toggle" || (postbackEvent[0] == "RichMenu" && postbackEvent[1] != "Help")
}

func ProcessPostbackEvent(
    event *linebot.Event,
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

    // business and user are empty if event does not require auth
    // WARN: ensure event handlers that require auth are added to shouldAuth() list
    var business model.Business
    var user model.User
    if shouldAuth(dataSlice) {
        var hasUserCompletedAuth bool
        var err error
        hasUserCompletedAuth, userPtr, businessPtr, err := auth.ValidateUserAuthOrRequestAuth(event.ReplyToken, userId, userDao, businessDao, line, log)
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

        business = *businessPtr
        user = *userPtr
    }

    switch dataSlice[0] {
    // /[NewReview|AiReply]/GenerateAiReply/{REVIEW_ID}
    case "NewReview", "AiReply":
        if len(dataSlice) == 3 && dataSlice[1] == "GenerateAiReply" && !util.IsEmptyString(dataSlice[2]) {
            reviewId := _type.NewReviewId(dataSlice[2])
            err := handleGenerateAiReply(event.ReplyToken, user, business, reviewId, reviewDao, line, log)
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

                var err error
                switch dataSlice[2] {
                case "Emoji":
                    user, err = handleEmojiToggle(user, userDao, log)
                    if err != nil {
                        log.Errorf("Error handling emoji toggle: %s", err)

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
                    user, err = handleSignatureToggle(user, userDao, log)

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
                    business, err = handleKeywordToggle(user, business, businessDao)
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
                    user, err = handleServiceRecommendationToggle(user, userDao, log)
                    if err != nil {
                        log.Errorf("Error handling service recommendation toggle: %s", err)

                        if _, ok := err.(*exception.AutoQuickReplyConditionNotMetException); ok {
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

                err = line.ShowAiReplySettings(event.ReplyToken, user, business)
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
            err := line.ShowQuickReplySettings(event.ReplyToken, business.AutoQuickReplyEnabled, business.QuickReplyMessage)
            if err != nil {
                log.Errorf("Error sending quick reply settings to user '%s': %v", user.UserId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error sending quick reply settings: %s"}`, err),
                }, err
            }

        case "AiReplySettings":
            err := line.ShowAiReplySettings(event.ReplyToken, user, business)
            if err != nil {
                log.Errorf("Error sending AI reply settings to user '%s': %v", user.UserId, err)
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

            autoQuickReplyEnabled, quickReplyMessage, err := handleAutoQuickReplyToggle(user, business, businessDao)
            if err != nil {
                if _, ok := err.(*exception.AutoQuickReplyConditionNotMetException); ok {
                    log.Warnf("Auto reply condition not met for user '%s': %v", userId, err)
                    _, err := line.ReplyUser(event.ReplyToken, "請先填寫快速回覆訊息，才能開啟自動回覆功能")
                    if err != nil {
                        log.Errorf("Error replying cannot enable auto quick reply prompt to user '%s': %v", userId, err)
                        return events.LambdaFunctionURLResponse{
                            StatusCode: 00,
                            Body:       fmt.Sprintf(`{"error": "Error replying cannot enable auto quick reply prompt: %s"}`, err),
                        }, err
                    }
                    log.Warnf("Notified user '%s' to fill in quick reply message before enabling auto quick reply", userId)

                    return events.LambdaFunctionURLResponse{
                        StatusCode: 200,
                        Body:       fmt.Sprintf(`{"Rejected enabling auto quick reply feature": "Please fill in quick reply message before enabling auto quick reply"}`),
                    }, nil
                }

                log.Errorf("Error handling auto quick reply toggle for user '%s': %v", userId, err)

                _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "自動回覆")
                if err != nil {
                    log.Errorf("Error notifying user of updating auto quick reply enabled failed for user '%s': %v", userId, err)
                    return events.LambdaFunctionURLResponse{
                        StatusCode: 500,
                        Body:       fmt.Sprintf(`{"error": "Failed to notify user of updating auto quick reply enabled failed: %s"}`, err),
                    }, err
                }

                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error handling auto quick reply toggle: %s"}`, err),
                }, err
            }

            err = line.ShowQuickReplySettings(event.ReplyToken, autoQuickReplyEnabled, quickReplyMessage)
            if err != nil {
                log.Errorf("Error sending quick reply settings to user '%s': %v", userId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error sending quick reply settings: %s"}`, err),
                }, err
            }

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
