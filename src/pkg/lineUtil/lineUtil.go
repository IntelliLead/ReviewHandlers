package lineUtil

import (
    "errors"
    "fmt"
    jsonUtil2 "github.com/IntelliLead/CoreCommonUtil/jsonUtil"
    "github.com/IntelliLead/CoreCommonUtil/line"
    "github.com/IntelliLead/CoreCommonUtil/metric"
    "github.com/IntelliLead/CoreCommonUtil/metric/enum"
    "github.com/IntelliLead/CoreCommonUtil/stringUtil"
    "github.com/IntelliLead/CoreDataAccess/ddbDao"
    "github.com/IntelliLead/CoreDataAccess/model"
    "github.com/IntelliLead/CoreDataAccess/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
    "io"
    "net/http"
    "strings"
)

var (
    log *zap.SugaredLogger
)

type LineUtil struct {
    Base               line.Line
    reviewMessageJsons jsonUtil.ReviewMessageLineFlexTemplateJsons
    quickReplyJsons    jsonUtil.QuickReplySettingsLineFlexTemplateJsons
    aiReplyJsons       jsonUtil.AiReplyLineFlexTemplateJsons
    authJsons          jsonUtil.AuthLineFlexTemplateJsons
    notificationJsons  jsonUtil.NotificationLineFlexTemplateJsons
}

func NewLineUtil(lineChannelSecret string, lineChannelAccessToken string, logger *zap.SugaredLogger) *LineUtil {
    log = logger
    return &LineUtil{
        Base:               line.NewLine(lineChannelSecret, lineChannelAccessToken, logger),
        reviewMessageJsons: jsonUtil.LoadReviewMessageLineFlexTemplateJsons(),
        quickReplyJsons:    jsonUtil.LoadQuickReplySettingsLineFlexTemplateJsons(),
        aiReplyJsons:       jsonUtil.LoadAiReplyLineFlexTemplateJsons(),
        authJsons:          jsonUtil.LoadAuthLineFlexTemplateJsons(),
        notificationJsons:  jsonUtil.LoadNotificationLineFlexTemplateJsons(),
    }
}

func (l LineUtil) ReplyUnknownResponseReply(replyToken string) error {
    reviewMessage := fmt.Sprintf("對不起，我還不會處理您的訊息。如需幫助，請回覆\"/help\"")

    message := linebot.NewTextMessage(reviewMessage).WithQuickReplies(linebot.NewQuickReplyItems(
        // label must not be longer than 20 characters
        linebot.NewQuickReplyButton(
            "",
            linebot.NewMessageAction("幫助", "/help"),
        ),
    ))

    err := l.Base.ReplyMessage(replyToken, message)
    if err != nil {
        log.Error("Error sending message to line: ", err)
        return err
    }

    return nil
}

// SendNewReview sends a new review to all the users of the business
func (l LineUtil) SendNewReview(review model.Review, business model.Business, userDao *ddbDao.UserDao) error {
    quickReplyMessage := ""
    if !stringUtil.IsEmptyStringPtr(business.QuickReplyMessage) {
        quickReplyMessage = business.GetFinalQuickReplyMessage(review)
    }

    var returnErr error = nil
    for _, userId := range business.UserIds {
        // get the businessID for each user
        userPtr, err := userDao.GetUser(userId)
        if err != nil {
            log.Error("Error getting user in SendNewReview: ", err)
            return err
        }
        if userPtr == nil {
            log.Errorf("User '%s' not found. Skipping", userId)
            returnErr = errors.New(fmt.Sprintf("User '%s' not found", userId))
            continue
        }
        user := *userPtr

        businessIdIndex := stringUtil.FindStringIndex(bid.BusinessIdsToStringSlice(user.BusinessIds), business.BusinessId.String())
        if businessIdIndex < 0 {
            errStr := fmt.Sprintf("Business '%s' not found in user '%s'. Not sending review to this user.", business.BusinessId, userId)
            log.Error(errStr)
            metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameNewReviewEventHandler.String(), 1)
            continue
        }

        // send the message to each user
        // omit business name if the user only has single business
        businessNamePtr := (*string)(nil)
        if len(user.BusinessIds) > 1 {
            businessNamePtr = &business.BusinessName
        }
        flexMessage, err := l.buildReviewFlexMessage(review, quickReplyMessage, business.BusinessId, businessIdIndex, businessNamePtr)
        if err != nil {
            log.Error("Error building flex message in SendNewReview: ", err)
        }

        err = l.Base.SendFlexMessage(userId, linebot.NewFlexMessage("您有新的Google Map 評論！", flexMessage))
        if err != nil {
            log.Errorf("Error sending lineTextMessage to LINE user %s in SendNewReview: %v", userId, err)
            returnErr = err
            continue
        }
    }

    return returnErr
}

// SendNewReview sends a new review to all the users of the business
// TODO: [INT-97] Remove this method when all users are backfilled with business IDs
func (l LineUtil) SendNewReviewToUser(review model.Review, userId string) error {
    flexMessage, err := l.buildReviewFlexMessageForUnauthedUser(review)
    if err != nil {
        log.Error("Error building flex message in SendNewReview: ", err)
    }

    return l.Base.SendFlexMessage(userId, linebot.NewFlexMessage("您有新的Google Map 評論！", flexMessage))
}

func (l LineUtil) ShowQuickReplySettings(replyToken string, user model.User, businessDao *ddbDao.BusinessDao) error {
    orderedBusinesses := make([]model.Business, len(user.BusinessIds))
    for i, id := range user.GetSortedBusinessIds() {
        b, err := businessDao.GetBusiness(id)
        if err != nil {
            log.Errorf("Error getting business '%s' for user '%s': %v", id, user.UserId, err)
            return err
        }
        if b == nil {
            log.Errorf("Business '%s' does not exist for user '%s'", id, user.UserId)
            return fmt.Errorf("business '%s' does not exist for user '%s'", id, user.UserId)
        }
        orderedBusinesses[i] = *b
    }

    if len(user.BusinessIds) > 1 {
        return l.showQuickReplySettingsForMultiBusiness(replyToken, orderedBusinesses, user.ActiveBusinessId)
    } else {
        return l.showQuickReplySettingsForSingleBusiness(replyToken, orderedBusinesses[0])
    }
}

func (l LineUtil) ShowQuickReplySettingsWithActiveBusiness(
    replyToken string,
    user model.User,
    activeBusiness model.Business,
    businessDao *ddbDao.BusinessDao,
) error {
    if len(user.BusinessIds) == 1 {
        return l.showQuickReplySettingsForSingleBusiness(replyToken, activeBusiness)
    }

    orderedBusinesses := make([]model.Business, len(user.BusinessIds))
    for i, id := range user.GetSortedBusinessIds() {
        if id == activeBusiness.BusinessId {
            orderedBusinesses[i] = activeBusiness
            continue
        }

        b, err := businessDao.GetBusiness(id)
        if err != nil {
            log.Errorf("Error getting business '%s' for user '%s': %v", id, user.UserId, err)
            return err
        }
        if b == nil {
            log.Errorf("Business '%s' does not exist for user '%s'", id, user.UserId)
            return fmt.Errorf("business '%s' does not exist for user '%s'", id, user.UserId)
        }
        orderedBusinesses[i] = *b
    }

    if user.ActiveBusinessId != activeBusiness.BusinessId {
        log.Errorf("Active business '%s' does not match business '%s' for user '%s'", user.ActiveBusinessId, activeBusiness.BusinessId, user.UserId)
        metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
    }

    return l.showQuickReplySettingsForMultiBusiness(replyToken, orderedBusinesses, activeBusiness.BusinessId)
}

func (l LineUtil) showQuickReplySettingsForMultiBusiness(
    replyToken string,
    orderedBusinesses []model.Business,
    activeBusinessId bid.BusinessId,
) error {
    flexMessage, err := l.buildQuickReplySettingsFlexMessageForMultiBusiness(
        orderedBusinesses,
        activeBusinessId,
    )
    if err != nil {
        log.Error("Error building flex message in showQuickReplySettingsForMultiBusiness: ", err)
    }

    if replyToken == util.TestReplyToken {
        return l.Base.SendFlexMessage("Ucc29292b212e271132cee980c58e94eb", linebot.NewFlexMessage("設定快速回覆", flexMessage))
    } else {
        return l.Base.ReplyFlexMessage(replyToken, linebot.NewFlexMessage("設定快速回覆", flexMessage))
    }
}

func (l LineUtil) showQuickReplySettingsForSingleBusiness(replyToken string, business model.Business) error {
    flexMessage, err := l.buildQuickReplySettingsFlexMessage(business)
    if err != nil {
        log.Error("Error building flex message in showQuickReplySettingsForSingleBusiness: ", err)
        return err
    }

    if replyToken == util.TestReplyToken {
        return l.Base.SendFlexMessage("Ucc29292b212e271132cee980c58e94eb", linebot.NewFlexMessage("設定快速回覆", flexMessage))
    } else {
        return l.Base.ReplyFlexMessage(replyToken, linebot.NewFlexMessage("設定快速回覆", flexMessage))
    }
}

func (l LineUtil) ShowAiReplySettingsByUser(replyToken string, user model.User, businessDao *ddbDao.BusinessDao) error {
    businessId := user.ActiveBusinessId
    businessPtr, err := businessDao.GetBusiness(businessId)
    if err != nil {
        log.Error("Error getting business in ShowAiReplySettingsByUser: ", err)
        return err
    }
    if businessPtr == nil {
        log.Errorf("Business '%s' not found", businessId)
        return errors.New(fmt.Sprintf("Business '%s' not found", businessId))
    }
    business := *businessPtr

    return l.ShowAiReplySettings(replyToken, user, business, businessDao)
}

func (l LineUtil) ShowAiReplySettings(
    replyToken string,
    user model.User,
    activeBusiness model.Business,
    businessDao *ddbDao.BusinessDao) error {
    if len(user.BusinessIds) > 1 {
        orderedBusinesses := make([]model.Business, len(user.BusinessIds))
        for i, id := range user.GetSortedBusinessIds() {
            b, err := businessDao.GetBusiness(id)
            if err != nil {
                log.Errorf("Error getting business '%s' for user '%s': %v", id, user.UserId, err)
                return err
            }
            if b == nil {
                log.Errorf("Business '%s' does not exist for user '%s'", id, user.UserId)
                return fmt.Errorf("business '%s' does not exist for user '%s'", id, user.UserId)
            }
            orderedBusinesses[i] = *b
        }
        if user.ActiveBusinessId != activeBusiness.BusinessId {
            log.Errorf("Active business '%s' does not match business '%s' for user '%s'", user.ActiveBusinessId, activeBusiness.BusinessId, user.UserId)
            metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
        }
        return l.showAiReplySettingsForMultiBusiness(replyToken, user, orderedBusinesses, activeBusiness.BusinessId)
    }

    return l.showAiReplySettingsForSingleBusiness(replyToken, user, activeBusiness)
}

func (l LineUtil) showAiReplySettingsForSingleBusiness(replyToken string, user model.User, business model.Business) error {
    flexMessage, err := l.buildAiReplySettingsFlexMessageForSingleBusiness(user, business)
    if err != nil {
        log.Error("Error building flex message in showAiReplySettingsForSingleBusiness: ", err)
    }

    if replyToken == util.TestReplyToken {
        err = l.Base.SendFlexMessage("Ucc29292b212e271132cee980c58e94eb", linebot.NewFlexMessage("AI 回覆設定", flexMessage))
    } else {
        err = l.Base.ReplyFlexMessage(replyToken, linebot.NewFlexMessage("AI 回覆設定", flexMessage))
    }
    if err != nil {
        return err
    }

    return nil
}

func (l LineUtil) showAiReplySettingsForMultiBusiness(replyToken string, user model.User, orderedBusinesses []model.Business, activeBusinessId bid.BusinessId) error {
    flexMessage, err := l.buildAiReplySettingsFlexMessageForMultiBusiness(user, orderedBusinesses, activeBusinessId)
    if err != nil {
        log.Error("Error building flex message in buildAiReplySettingsFlexMessageForMultiBusiness: ", err)
    }

    if replyToken == util.TestReplyToken {
        return l.Base.SendFlexMessage("Ucc29292b212e271132cee980c58e94eb", linebot.NewFlexMessage("AI 回覆設定", flexMessage))
    } else {
        return l.Base.ReplyFlexMessage(replyToken, linebot.NewFlexMessage("AI 回覆設定", flexMessage))
    }
}

func (l LineUtil) SendAiGeneratedReply(aiReply string, review model.Review, generateAuthorName string, business model.Business, user model.User, userDao *ddbDao.UserDao) error {
    var returnErr error = nil
    // for each user of the business, retrieve businessId Index for the user, and send the message
    for _, userId := range business.UserIds {
        var businessIdIndex int
        var err error
        // user already retrieved
        if userId == user.UserId {
            businessIdIndex, err = user.GetBusinessIdIndex(business.BusinessId)
            if err != nil {
                log.Errorf("Error getting businessIdIndex for business '%s' in SendAiGeneratedReply: %v", business.BusinessId, err)
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
                returnErr = err
                continue
            }
        } else {
            sendingUser, err := userDao.GetUser(userId)
            if err != nil {
                log.Errorf("Error getting user '%s' in SendAiGeneratedReply: %v", userId, err)
                return err
            }
            if sendingUser == nil {
                errMsg := fmt.Sprintf("User '%s' not found in SendAiGeneratedReply. Inconsistent userIds in business '%s'", userId, business.BusinessId)
                log.Error(errMsg)
                returnErr = errors.New(errMsg)
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
                continue
            }
            businessIdIndex, err = sendingUser.GetBusinessIdIndex(business.BusinessId)
            if err != nil {
                errMsg := fmt.Sprintf("Error getting businessIdIndex for business '%s' in user '%s' during SendAiGeneratedReply: %v", business.BusinessId, sendingUser.UserId, err)
                log.Error(errMsg)
                returnErr = errors.New(errMsg)
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
                continue
            }
        }

        flexMessage, err := l.buildAiGeneratedReplyFlexMessage(review, aiReply, generateAuthorName, business.BusinessId, businessIdIndex)
        if err != nil {
            log.Error("Error building flex message in SendAiGeneratedReply: ", err)
            returnErr = err
            metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
            continue
        }

        err = l.Base.SendFlexMessage(userId, linebot.NewFlexMessage("AI 回覆生成結果", flexMessage))
        if err != nil {
            log.Errorf("Error sending lineTextMessage to LINE user %s in SendAiGeneratedReply: %v", userId, err)
            returnErr = err
            metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
            continue
        }
    }

    return returnErr
}

func (l LineUtil) SendAuthRequest(userId string, authRedirectUrl string) error {
    flexMessage, err := l.buildAuthRequestFlexMessage(userId, authRedirectUrl)
    if err != nil {
        log.Error("Error building flex message in RequestAuth: ", err)
        return err
    }

    return l.Base.SendFlexMessage(userId, linebot.NewFlexMessage("智引力請求訪問 Google 資料", flexMessage))
}

func (l LineUtil) ReplyAuthRequest(replyToken string, userId string, authRedirectUrl string) error {
    flexMessage, err := l.buildAuthRequestFlexMessage(userId, authRedirectUrl)
    if err != nil {
        log.Error("Error building flex message in RequestAuth: ", err)
        return err
    }

    return l.Base.ReplyFlexMessage(replyToken, linebot.NewFlexMessage("智引力請求訪問 Google 資料", flexMessage))
}

func (l LineUtil) ReplyUserReplyFailed(replyToken string, reviewerName string, isAutoReply bool) error {
    return l.Base.ReplyText(replyToken, buildReplyFailedMessage(reviewerName, isAutoReply))
}

func (l LineUtil) NotifyUsersReplyFailed(userIds []string, reviewerName string, isAutoReply bool) error {
    var returnErr error = nil
    for _, userId := range userIds {
        err := l.Base.SendText(userId, buildReplyFailedMessage(reviewerName, isAutoReply))
        if err != nil {
            log.Errorf("Error sending message to '%s' in NotifyUsersReplyFailed: %v", userId, err)
            returnErr = err
        }
    }
    return returnErr
}

// ReplyUserReplyFailedWithReason replies to the user that the reply failed with the reason
// both the reviewerName and reason can be empty
func (l LineUtil) ReplyUserReplyFailedWithReason(replyToken string, reviewerName string, reason string) error {
    var text string
    if stringUtil.IsEmptyString(reviewerName) {
        text = "回覆評論失敗。"
    } else {
        text = fmt.Sprintf("回覆 %s 的評論失敗。", reviewerName)
    }
    text += reason + "很抱歉為您造成不便。"

    return l.Base.ReplyText(replyToken, text)
}

// NotifyReviewAutoReplied notifies all users of the business that owns the review that the review has been replied to
// param review: the review that was replied to
// param reply: the reply to the review
// param business: the business that owns the review
// param userDao: the userDao
func (l LineUtil) NotifyReviewAutoReplied(
    review model.Review,
    reply string,
    business model.Business,
    userDao *ddbDao.UserDao,
) error {
    var returnErr error = nil
    for _, userId := range business.UserIds {
        sendingUser, err := userDao.GetUser(userId)
        if err != nil {
            log.Errorf("Error getting user '%s' in NotifyReviewReplied: %v", userId, err)
            return err
        }
        if sendingUser == nil {
            errMsg := fmt.Sprintf("User '%s' not found in NotifyReviewReplied. Inconsistent userIds in business '%s'", userId, business.BusinessId)
            log.Error(errMsg)
            metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
            returnErr = errors.New(errMsg)
            continue
        }
        businessIdIndex, err := sendingUser.GetBusinessIdIndex(business.BusinessId)
        if err != nil {
            errMsg := fmt.Sprintf("Error getting businessIdIndex for business '%s' in user '%s' during NotifyReviewReplied: %v", business.BusinessId, sendingUser.UserId, err)
            log.Error(errMsg)
            metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
            returnErr = errors.New(errMsg)
            continue
        }

        flexMessage, err := l.buildReviewRepliedNotificationMessage(review, reply, "自動回覆", true, business.BusinessName, businessIdIndex)
        if err != nil {
            log.Error("Error building flex message in NotifyReviewReplied: ", err)
            return err
        }

        err = l.Base.SendFlexMessage(userId, linebot.NewFlexMessage("評論回覆通知", flexMessage))
        if err != nil {
            errMsg := fmt.Sprintf("Error sending message to '%s' in NotifyReviewReplied: %v . Flex Message: %s", userId, err, jsonUtil2.AnyToJson(flexMessage))
            log.Error(errMsg)
            metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
            returnErr = errors.New(errMsg)
            continue
        }

        log.Infof("Successfully sent auto reply notification to user '%s'", userId)
    }

    return returnErr
}

// NotifyReviewReplied notifies all users of the business that owns the review that the review has been replied to
// param replyToken: the reply token of the user who replied to the review
// param replyTokenOwnerUserId: the userId of the user who replied to the review
// param review: the review that was replied to
// param business: the business that owns the review
// param replierUser: the user who replied to the review
// param userDao: the userDao
func (l LineUtil) NotifyReviewReplied(
    replyToken string,
    review model.Review,
    reply string,
    business model.Business,
    replierUser model.User,
    userDao *ddbDao.UserDao,
) error {
    var returnErr error = nil
    for _, userId := range business.UserIds {
        if !stringUtil.IsEmptyString(replyToken) && userId == replierUser.UserId && replyToken != util.TestReplyToken {
            log.Infof("Sending reply message to reply token owner user '%s'", replierUser.UserId)
            businessIdIndex, err := replierUser.GetBusinessIdIndex(business.BusinessId)
            if err != nil {
                log.Errorf("Error getting businessIdIndex for business '%s' in user '%s' during NotifyReviewReplied: %v", business.BusinessId, replierUser.UserId, err)
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
                returnErr = err
                continue
            }
            flexMessage, err := l.buildReviewRepliedNotificationMessage(review, reply, replierUser.LineUsername, false, business.BusinessName, businessIdIndex)
            if err != nil {
                log.Error("Error building flex message in NotifyReviewReplied: ", err)
                return err
            }
            err = l.Base.ReplyFlexMessage(replyToken, linebot.NewFlexMessage("評論回覆通知", flexMessage))
            if err != nil {
                log.Errorf("Error sending message to '%s' in NotifyReviewReplied: %v . Flex Message: %s", userId, err, jsonUtil2.AnyToJson(flexMessage))
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
                returnErr = err
                continue
            }
        } else {
            sendingUser, err := userDao.GetUser(userId)
            if err != nil {
                log.Errorf("Error getting user '%s' in NotifyReviewReplied: %v", userId, err)
                return err
            }
            if sendingUser == nil {
                errStr := fmt.Sprintf("User '%s' not found in NotifyReviewReplied. Inconsistent userIds in business '%s'", userId, business.BusinessId)
                log.Error(errStr)
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
                returnErr = errors.New(errStr)
                continue
            }
            businessIdIndex, err := sendingUser.GetBusinessIdIndex(business.BusinessId)
            if err != nil {
                log.Errorf("Error getting businessIdIndex for business '%s' in user '%s' during NotifyReviewReplied: %v", business.BusinessId, sendingUser.UserId, err)
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
                returnErr = err
                continue
            }

            flexMessage, err := l.buildReviewRepliedNotificationMessage(review, reply, replierUser.LineUsername, false, business.BusinessName, businessIdIndex)
            if err != nil {
                log.Error("Error building flex message in NotifyReviewReplied: ", err)
                return err
            }

            err = l.Base.SendFlexMessage(userId, linebot.NewFlexMessage("評論回覆通知", flexMessage))
            if err != nil {
                log.Errorf("Error sending message to '%s' in NotifyReviewReplied: %v . Flex Message: %s", userId, err, jsonUtil2.AnyToJson(flexMessage))
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler.String(), 1)
                returnErr = err
                continue
            }
        }

        log.Infof("Successfully executed line.PushMessage/ReplyText in NotifyReviewReplied to user '%s'", userId)
    }

    return returnErr
}

// NotifyUserUpdateFailed let user know that the update failed
// param updateType: is the Mandarin text of the update type in notification
// Example: 快速回覆訊息, 關鍵字, 主要業務
func (l LineUtil) NotifyUserUpdateFailed(replyToken string, updateType string) error {
    return l.Base.ReplyText(replyToken, fmt.Sprintf("%s更新失敗，請稍後再試。很抱歉為您造成不便。", updateType))
}

func (l LineUtil) NotifyUserAiReplyGenerationInProgress(replyToken string) error {
    return l.Base.ReplyText(replyToken, "AI 回覆生成中…")
}

func (l LineUtil) NotifyUserAiReplyGenerationFailed(userId string) error {
    return l.Base.SendText(userId, "AI 回覆生成失敗，請稍後再試。很抱歉為您造成不便。")
}

func (l LineUtil) ReplyHelpMessage(replyToken string) error {
    return l.Base.ReplyText(replyToken, util.HelpMessage())
}

func (l LineUtil) ReplyMoreMessage(replyToken string) error {
    return l.Base.ReplyText(replyToken, util.MoreMessage())
}

func (l LineUtil) NotifyUserCannotUseLineEmoji(replyToken string) error {
    return l.Base.ReplyText(replyToken, CannotUseLineEmojiMessage)
}

func (l LineUtil) ParseRequest(request *events.LambdaFunctionURLRequest) ([]*linebot.Event, error) {
    httpRequest := convertToHttpRequest(request)
    return l.Base.LineClient.ParseRequest(httpRequest)
}

func convertToHttpRequest(request *events.LambdaFunctionURLRequest) *http.Request {
    // Create a new http.Request with headers and body from LambdaFunctionURLRequest
    headers := http.Header{}
    for k, v := range (*request).Headers {
        headers.Set(k, v)
    }
    return &http.Request{
        Method: request.RequestContext.HTTP.Method,
        URL:    nil,
        Header: headers,
        Body:   io.NopCloser(strings.NewReader(request.Body)),
    }
}

func (l LineUtil) NotifyQuickReplySettingsUpdated(userIds []string, updaterName string, businessName string) error {
    flexMessage, err := l.buildQuickReplySettingsUpdatedNotificationMessage(updaterName, businessName)
    if err != nil {
        log.Error("Error building flex message in NotifyQuickReplySettingsUpdated: ", err)
        return err
    }

    var returnErr error = nil
    for _, userId := range userIds {
        err = l.Base.SendFlexMessage(userId, linebot.NewFlexMessage("快速回覆設定更新通知", flexMessage))
        if err != nil {
            errMsg := fmt.Sprintf("Error sending message to '%s' in NotifyQuickReplySettingsUpdated: %v", userId, err)
            log.Error(errMsg)
            returnErr = errors.New(errMsg)
        } else {
            log.Infof("Successfully executed line.PushMessage in NotifyQuickReplySettingsUpdated to user '%s'", userId)
        }
    }

    return returnErr
}

func (l LineUtil) NotifyAiReplySettingsUpdated(userIds []string, updaterName string, businessName string) error {
    flexMessage, err := l.buildAiReplySettingsUpdatedNotificationMessage(updaterName, businessName)
    if err != nil {
        log.Error("Error building flex message in NotifyAiReplySettingsUpdated: ", err)
        return err
    }

    var returnErr error = nil
    for _, userId := range userIds {
        err = l.Base.SendFlexMessage(userId, linebot.NewFlexMessage("AI回覆設定更新通知", flexMessage))
        if err != nil {
            errMsg := fmt.Sprintf("Error sending message to '%s' in NotifyAiReplySettingsUpdated: %v", userId, err)
            log.Error(errMsg)
            returnErr = errors.New(errMsg)
        } else {
            log.Infof("Successfully executed line.PushMessage in NotifyAiReplySettingsUpdated to user '%s'", userId)
        }
    }

    return returnErr
}
