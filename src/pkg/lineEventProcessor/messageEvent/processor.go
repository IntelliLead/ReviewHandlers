package messageEvent

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/auth"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
)

func shouldAuth(message string) bool {
    cmdMsg := lineUtil.ParseCommandMessage(message, false)

    return lineUtil.IsReviewReplyMessage(message) || (cmdMsg.Command != "h" && cmdMsg.Command != "Help" && cmdMsg.Command != "help" && cmdMsg.Command != "幫助" && cmdMsg.Command != "協助")
}

// ProcessMessageEvent processes a message event from LINE
// It returns a LambdaFunctionURLResponse and an error
func ProcessMessageEvent(
    event *linebot.Event,
    userId string,
    businessDao *ddbDao.BusinessDao,
    userDao *ddbDao.UserDao,
    reviewDao *ddbDao.ReviewDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) (events.LambdaFunctionURLResponse, error) {

    // validate is message from user
    // --------------------------------
    isMessageFromUser := lineUtil.IsMessageFromUser(event)
    if !isMessageFromUser {
        log.Info("Not a message from user. Ignoring.")
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Not a text message from user. Ignoring."}`,
        }, nil
    }

    // validate is text message from user
    // --------------------------------
    isTextMessageFromUser, err := lineUtil.IsTextMessage(event)
    if err != nil {
        log.Error("Error checking if event is text message from user:", err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to check if event is text message from user: %s"}`, err),
        }, err
    }

    if !isTextMessageFromUser {
        log.Info("Message from user is not a text message.")

        err := line.ReplyUnknownResponseReply(event.ReplyToken)
        if err != nil {
            log.Error("Error executing ReplyUnknownResponseReply: ", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error executing ReplyUnknownResponseReply: %s"}`, err),
            }, err
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Message from user is not a text message."}`,
        }, nil
    }

    textMessage := event.Message.(*linebot.TextMessage)
    message := textMessage.Text
    log.Infof("Received text message from user '%s': %s", userId, message)

    // process review reply request
    // --------------------------------

    // business and user are empty if event does not require auth
    // WARN: ensure event handlers that require auth are added to shouldAuth() list
    var business model.Business
    var user model.User
    if shouldAuth(message) {
        var hasUserAuthed bool
        hasUserAuthed, userPtr, businessPtr, err := auth.ValidateUserAuthOrRequestAuth(event.ReplyToken, userId, userDao, businessDao, line, log)
        if err != nil {
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to validate user auth: %s"}`, err),
            }, err
        }
        if !hasUserAuthed {
            return events.LambdaFunctionURLResponse{
                StatusCode: 200,
                Body:       `{"message": "User has not authenticated. Requested authentication."}`,
            }, nil
        }

        user = *userPtr
        business = *businessPtr
    }

    if lineUtil.IsReviewReplyMessage(message) {
        return ProcessReviewReplyMessage(business, user, event, reviewDao, line, log)
    }

    // process command requests
    cmdMsg := lineUtil.ParseCommandMessage(message, false)
    args := ""
    if len(cmdMsg.Args) > 0 {
        args = cmdMsg.Args[0]
    }
    switch cmdMsg.Command {
    case "h", "Help", "help", "幫助", "協助":
        _, err := line.ReplyHelpMessage(event.ReplyToken)
        if err != nil {
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to reply help message: %s"}`, err),
            }, err
        }

        log.Infof("Successfully processed help request to user '%s'", userId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Successfully processed help request"}`,
        }, nil

    case "q", util.UpdateQuickReplyMessageCmd, "快速回覆":

        // validate message does not contain LINE emojis
        // --------------------------------
        if HasLineEmoji(textMessage) {
            _, err := line.NotifyUserCannotUseLineEmoji(event.ReplyToken)
            if err != nil {
                log.Errorf("Error notifying user '%s' that LINE Emoji is not yet supported for quick reply: %v", userId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to notify user of LINE Emoji not yet supported: %s"}`, err),
                }, err
            }

            return events.LambdaFunctionURLResponse{
                StatusCode: 200,
                Body:       `{"message": "Notified LINE Emoji not yet supported"}`,
            }, nil
        }

        quickReplyMessage := args

        autoQuickReplyEnabled, storedQuickReplyMessage, err := handleUpdateQuickReply(user, quickReplyMessage, businessDao, log)
        if err != nil {
            log.Errorf("Error updating quick reply message '%s' for user '%s': %v", quickReplyMessage, userId, err)
            _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "快速回覆訊息")
            if err != nil {
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to notify user of update quick reply message failed: %s"}`, err),
                }, err
            }
            log.Error("Successfully notified user of update quick reply message failed")

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to update quick reply message: %s"}`, err),
            }, err
        }

        // notify all other users of toggle (skip notifying self)
        err = line.NotifyQuickReplySettingsUpdated(util.RemoveStringFromSlice(business.UserIds, userId), user.LineUsername)
        if err != nil {
            log.Errorf("Error notifying other users of quick reply settings update for user '%s': %v", userId, err)
        }

        err = line.ShowQuickReplySettings(event.ReplyToken, autoQuickReplyEnabled, storedQuickReplyMessage)
        if err != nil {
            log.Errorf("Error showing quick reply settings for user '%s': %v", userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to show quick reply settings: %s"}`, err),
            }, err
        }

        log.Infof("Successfully processed update quick reply message request for user '%s'", userId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Successfully processed update quick reply message request"}`,
        }, nil

    case "d", util.UpdateBusinessDescriptionMessageCmd, "主要業務":
        // process update quick reply message request
        businessDescription := args

        possiblyUpdatedUser, updatedBusiness, err := handleBusinessDescriptionUpdate(user, businessDescription, userDao, businessDao, log)
        if err != nil {
            log.Errorf("Error updating business description '%s' for user '%s': %v", businessDescription, userId, err)

            _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "主要業務")
            if err != nil {
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to notify user of update business description failed: %s"}`, err),
                }, err
            }
            log.Error("Successfully notified user of update signature failed")
        }

        // notify all other users of toggle (skip notifying self)
        err = line.NotifyAiReplySettingsUpdated(util.RemoveStringFromSlice(business.UserIds, userId), user.LineUsername)
        if err != nil {
            log.Errorf("Error notifying other users of AI reply settings update for user '%s': %v", userId, err)
        }

        err = line.ShowAiReplySettings(event.ReplyToken, possiblyUpdatedUser, updatedBusiness)
        if err != nil {
            log.Errorf("Error showing AI reply settings for user '%s': %v", userId, err)

            _, err := line.ReplyUser(event.ReplyToken, "主要業務更新成功，但顯示設定失敗，請稍後再試")
            if err != nil {
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to reply user of update business description success but show settings failed: %s"}`, err),
                }, err
            }
            log.Error("Successfully replied user of update business description success but show settings failed")

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to show AI reply settings: %s"}`, err),
            }, err
        }

        log.Infof("Successfully processed update business description request for user '%s'", userId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Successfully processed update business description request"}`,
        }, nil

    case "s", util.UpdateSignatureMessageCmd, "簽名":
        // process update quick reply message request
        signature := args

        updatedUser, err := handleUpdateSignature(user, signature, userDao, log)
        if err != nil {
            log.Errorf("Error updating signature '%s' for user '%s': %v", signature, userId, err)

            _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "簽名")
            if err != nil {
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to notify user of update signature failed: %s"}`, err),
                }, err
            }
            log.Error("Successfully notified user of update signature failed")

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to update signature: %s"}`, err),
            }, err
        }

        err = line.ShowAiReplySettings(event.ReplyToken, updatedUser, business)
        if err != nil {
            log.Errorf("Error showing AI reply settings for user '%s': %v", userId, err)

            _, err := line.ReplyUser(event.ReplyToken, "簽名更新成功，但顯示設定失敗，請稍後再試")
            if err != nil {
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to reply user of update signature success: %s"}`, err),
                }, err
            }
            log.Error("Successfully replied user of update signature success but show settings failed")

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to show AI reply settings: %s"}`, err),
            }, err
        }

        log.Infof("Successfully processed update signature request for user '%s'", userId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Successfully processed update signature request"}`,
        }, nil

    case "k", util.UpdateKeywordsMessageCmd, "關鍵字":
        // process update quick reply message request
        keywords := args

        updatedBusiness, err := handleUpdateKeywords(user, keywords, businessDao, log)
        if err != nil {
            log.Errorf("Error updating keywords '%s' for user '%s': %v", keywords, userId, err)

            _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "關鍵字")
            if err != nil {
                log.Errorf("Failed to notify user of update keywords failed: %v", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to notify user of update keywords failed: %s"}`, err),
                }, err
            }
            log.Error("Successfully notified user of update keywords failed")
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to update keywords: %s"}`, err),
            }, err
        }

        // notify all other users of toggle (skip notifying self)
        err = line.NotifyAiReplySettingsUpdated(util.RemoveStringFromSlice(business.UserIds, userId), user.LineUsername)
        if err != nil {
            log.Errorf("Error notifying other users of AI reply settings update for user '%s': %v", userId, err)
        }

        err = line.ShowAiReplySettings(event.ReplyToken, user, updatedBusiness)
        if err != nil {
            log.Errorf("Error showing AI reply settings for user '%s': %v", userId, err)

            _, err := line.ReplyUser(event.ReplyToken, "關鍵字更新成功，但顯示設定失敗，請稍後再試")
            if err != nil {
                log.Errorf("Failed to reply user of update keywords success: %v", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to reply user of update keywords success: %s"}`, err),
                }, err
            }
            log.Error("Successfully replied user of update keywords success but show settings failed")
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to show AI reply settings : %s"}`, err),
            }, err
        }

        log.Infof("Successfully processed update keywords request for user '%s'", userId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Successfully processed update keywords request"}`,
        }, nil

    case "r", util.UpdateRecommendationMessageCmd, "推薦":
        serviceRecommendation := args

        updatedUser, err := handleUpdateServiceRecommendation(user, serviceRecommendation, userDao)
        if err != nil {
            log.Errorf("Error updating service recommendation '%s' for user '%s': %v", serviceRecommendation, userId, err)

            _, err := line.NotifyUserUpdateFailed(event.ReplyToken, "推薦業務")
            if err != nil {
                log.Errorf("Failed to notify user of update service recommendation failed: %v", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to notify user of update service recommendation failed: %s"}`, err),
                }, err
            }
            log.Error("Successfully notified user of update service recommendation failed")
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to update service recommendation: %s"}`, err),
            }, err
        }

        // notify all other users of toggle (skip notifying self)
        err = line.NotifyAiReplySettingsUpdated(util.RemoveStringFromSlice(business.UserIds, userId), user.LineUsername)
        if err != nil {
            log.Errorf("Error notifying other users of AI reply settings update for user '%s': %v", userId, err)
        }

        err = line.ShowAiReplySettings(event.ReplyToken, updatedUser, business)
        if err != nil {
            log.Errorf("Error showing AI reply settings for user '%s': %v", userId, err)

            _, err := line.ReplyUser(event.ReplyToken, "推薦更新成功，但顯示設定失敗，請稍後再試")
            if err != nil {
                log.Errorf("Failed to reply user of update service recommendation success: %v", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to reply user of update service recommendation success: %s"}`, err),
                }, err
            }
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to show AI reply settings: %s"}`, err),
            }, err
        }

        log.Infof("Successfully processed update service recommendation request for user '%s'", userId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Successfully processed update service recommendation request"}`,
        }, nil

    default:
        // handle unknown message from user
        err = line.ReplyUnknownResponseReply(event.ReplyToken)
        if err != nil {
            log.Error("Error executing ReplyUnknownResponseReply: ", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error executing ReplyUnknownResponseReply: %s"}`, err),
            }, err
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Text message from user is not handled."}`,
        }, nil

    }
}
