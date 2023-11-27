package lineUtil

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/awsUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/metric"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/metric/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
    "io"
    "net/http"
    "strings"
)

type Line struct {
    lineClient         *linebot.Client
    log                *zap.SugaredLogger
    reviewMessageJsons jsonUtil.ReviewMessageLineFlexTemplateJsons
    quickReplyJsons    jsonUtil.QuickReplySettingsLineFlexTemplateJsons
    aiReplyJsons       jsonUtil.AiReplyLineFlexTemplateJsons
    authJsons          jsonUtil.AuthLineFlexTemplateJsons
    notificationJsons  jsonUtil.NotificationLineFlexTemplateJsons
}

func NewLine(logger *zap.SugaredLogger) *Line {
    return &Line{
        lineClient:         newLineClient(logger),
        log:                logger,
        reviewMessageJsons: jsonUtil.LoadReviewMessageLineFlexTemplateJsons(),
        quickReplyJsons:    jsonUtil.LoadQuickReplySettingsLineFlexTemplateJsons(),
        aiReplyJsons:       jsonUtil.LoadAiReplyLineFlexTemplateJsons(),
        authJsons:          jsonUtil.LoadAuthLineFlexTemplateJsons(),
        notificationJsons:  jsonUtil.LoadNotificationLineFlexTemplateJsons(),
    }
}

func (l *Line) GetUser(userId string) (linebot.UserProfileResponse, error) {
    resp, err := l.lineClient.GetProfile(userId).Do()
    if err != nil {
        l.log.Error("Error getting user profile: ", err)
        return linebot.UserProfileResponse{}, err
    }

    return *resp, nil
}

func (l *Line) SendMessage(userId string, message string) error {
    resp, err := l.lineClient.PushMessage(userId, linebot.NewTextMessage(message)).Do()
    if err != nil {
        l.log.Errorf("Error sending message to '%s': %s", userId, err)
        return err
    }

    l.log.Infof("Successfully sent message to user '%s': %s", userId, jsonUtil.AnyToJson(resp))
    return nil
}

func (l *Line) ReplyUnknownResponseReply(replyToken string) error {
    reviewMessage := fmt.Sprintf("對不起，我還不會處理您的訊息。如需幫助，請回覆\"/help\"")

    message := linebot.NewTextMessage(reviewMessage).WithQuickReplies(linebot.NewQuickReplyItems(
        // label` must not be longer than 20 characters
        linebot.NewQuickReplyButton(
            "",
            linebot.NewMessageAction("幫助", "/help"),
        ),
    ))

    resp, err := l.lineClient.ReplyMessage(replyToken, message).Do()
    if err != nil {
        l.log.Error("Error sending message to line: ", err)
        return err
    }

    l.log.Debugf("Successfully executed line.ReplyMessage in ReplyUnknownResponseReply: %s", jsonUtil.AnyToJson(resp))
    return nil
}

// SendNewReview sends a new review to all the users of the business
func (l *Line) SendNewReview(review model.Review, business model.Business, userDao *ddbDao.UserDao) error {
    quickReplyMessage := ""
    if !util.IsEmptyStringPtr(business.QuickReplyMessage) {
        quickReplyMessage = business.GetFinalQuickReplyMessage(review)
    }

    for _, userId := range business.UserIds {
        // get the businessID for each user
        userPtr, err := userDao.GetUser(userId)
        if err != nil {
            l.log.Error("Error getting user in SendNewReview: ", err)
            return err
        }
        if userPtr == nil {
            l.log.Errorf("User '%s' not found", userId)
            return errors.New(fmt.Sprintf("User '%s' not found", userId))
        }

        businessIdIndex := util.FindStringIndex(bid.BusinessIdsToStringSlice(userPtr.BusinessIds), business.BusinessId.String())
        if businessIdIndex == -1 {
            errStr := fmt.Sprintf("Business '%s' not found in user '%s'. Not sending review to this user.", business.BusinessId, userId)
            l.log.Error(errStr)
            metric.EmitLambdaMetric(enum.Metric4xxError, enum2.HandlerNameNewReviewEventHandler, 1)
            continue
        }

        // send the message to each user
        // omit business name if the user only has single business
        businessNamePtr := (*string)(nil)
        // if len(userPtr.BusinessIds) > 1 {
        //     businessNamePtr = &business.BusinessName
        // }
        flexMessage, err := l.buildReviewFlexMessage(review, quickReplyMessage, business.BusinessId, businessIdIndex, businessNamePtr)
        if err != nil {
            l.log.Error("Error building flex message in SendNewReview: ", err)
        }

        _, err = l.lineClient.PushMessage(userId, linebot.NewFlexMessage("您有新的Google Map 評論！", flexMessage)).Do()
        if err != nil {
            l.log.Errorf("Error sending lineTextMessage to LINE user %s in SendNewReview: %v", userId, err)
            return err
        }
        l.log.Infof("Successfully executed line.PushMessage to send review '%s' to business '%s' user '%s'.", review.ReviewId, business.BusinessId, userId)
    }

    return nil
}

// SendNewReview sends a new review to all the users of the business
// TODO: [INT-97] Remove this method when all users are backfilled with business IDs
func (l *Line) SendNewReviewToUser(review model.Review, userId string) error {
    flexMessage, err := l.buildReviewFlexMessageForUnauthedUser(review)
    if err != nil {
        l.log.Error("Error building flex message in SendNewReview: ", err)
    }

    _, err = l.lineClient.PushMessage(userId, linebot.NewFlexMessage("您有新的Google Map 評論！", flexMessage)).Do()
    if err != nil {
        l.log.Errorf("Error sending lineTextMessage to LINE user %s in SendNewReview: %v", userId, err)
        return err
    }

    return nil
}

func (l *Line) ShowQuickReplySettings(replyToken string, user model.User, businessDao *ddbDao.BusinessDao) error {
    orderedBusinesses := make([]model.Business, len(user.BusinessIds))
    for i, id := range user.GetSortedBusinessIds() {
        b, err := businessDao.GetBusiness(id)
        if err != nil {
            l.log.Errorf("Error getting business '%s' for user '%s': %v", id, user.UserId, err)
            return err
        }
        if b == nil {
            l.log.Errorf("Business '%s' does not exist for user '%s'", id, user.UserId)
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

func (l *Line) ShowQuickReplySettingsWithActiveBusiness(
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
            l.log.Errorf("Error getting business '%s' for user '%s': %v", id, user.UserId, err)
            return err
        }
        if b == nil {
            l.log.Errorf("Business '%s' does not exist for user '%s'", id, user.UserId)
            return fmt.Errorf("business '%s' does not exist for user '%s'", id, user.UserId)
        }
        orderedBusinesses[i] = *b
    }

    if user.ActiveBusinessId != activeBusiness.BusinessId {
        l.log.Errorf("Active business '%s' does not match business '%s' for user '%s'", user.ActiveBusinessId, activeBusiness.BusinessId, user.UserId)
        metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler, 1)
    }

    return l.showQuickReplySettingsForMultiBusiness(replyToken, orderedBusinesses, activeBusiness.BusinessId)
}

func (l *Line) showQuickReplySettingsForMultiBusiness(
    replyToken string,
    orderedBusinesses []model.Business,
    activeBusinessId bid.BusinessId,
) error {
    flexMessage, err := l.buildQuickReplySettingsFlexMessageForMultiBusiness(
        orderedBusinesses,
        activeBusinessId,
    )
    if err != nil {
        l.log.Error("Error building flex message in showQuickReplySettingsForMultiBusiness: ", err)
    }

    var resp *linebot.BasicResponse
    if replyToken == util.TestReplyToken {
        resp, err = l.lineClient.PushMessage("Ucc29292b212e271132cee980c58e94eb", linebot.NewFlexMessage("設定快速回覆", flexMessage)).Do()
    } else {
        resp, err = l.lineClient.ReplyMessage(replyToken, linebot.NewFlexMessage("設定快速回覆", flexMessage)).Do()
    }
    if err != nil {
        l.log.Error("Error replying message in ShowQuickReplySettings: ", err)
        return err
    }

    l.log.Debugf("Successfully executed line.ReplyMessage in ShowQuickReplySettings: %s", jsonUtil.AnyToJson(resp))

    return nil
}

func (l *Line) showQuickReplySettingsForSingleBusiness(replyToken string, business model.Business) error {
    flexMessage, err := l.buildQuickReplySettingsFlexMessage(business)
    if err != nil {
        l.log.Error("Error building flex message in showQuickReplySettingsForSingleBusiness: ", err)
    }

    var resp *linebot.BasicResponse
    if replyToken == util.TestReplyToken {
        resp, err = l.lineClient.PushMessage("Ucc29292b212e271132cee980c58e94eb", linebot.NewFlexMessage("設定快速回覆", flexMessage)).Do()
    } else {
        resp, err = l.lineClient.ReplyMessage(replyToken, linebot.NewFlexMessage("設定快速回覆", flexMessage)).Do()
    }
    if err != nil {
        l.log.Error("Error replying message in showQuickReplySettingsForSingleBusiness: ", err)
        return err
    }

    l.log.Debugf("Successfully executed line.ReplyMessage in showQuickReplySettingsForSingleBusiness: %s", jsonUtil.AnyToJson(resp))

    return nil
}

func (l *Line) ShowAiReplySettingsByUser(replyToken string, user model.User, businessDao *ddbDao.BusinessDao) error {
    businessId := user.ActiveBusinessId
    businessPtr, err := businessDao.GetBusiness(businessId)
    if err != nil {
        l.log.Error("Error getting business in ShowAiReplySettingsByUser: ", err)
        return err
    }
    if businessPtr == nil {
        l.log.Errorf("Business '%s' not found", businessId)
        return errors.New(fmt.Sprintf("Business '%s' not found", businessId))
    }
    business := *businessPtr

    return l.ShowAiReplySettings(replyToken, user, business, businessDao)
}

func (l *Line) ShowAiReplySettings(
    replyToken string,
    user model.User,
    activeBusiness model.Business,
    businessDao *ddbDao.BusinessDao) error {
    if len(user.BusinessIds) > 1 {
        orderedBusinesses := make([]model.Business, len(user.BusinessIds))
        for i, id := range user.GetSortedBusinessIds() {
            b, err := businessDao.GetBusiness(id)
            if err != nil {
                l.log.Errorf("Error getting business '%s' for user '%s': %v", id, user.UserId, err)
                return err
            }
            if b == nil {
                l.log.Errorf("Business '%s' does not exist for user '%s'", id, user.UserId)
                return fmt.Errorf("business '%s' does not exist for user '%s'", id, user.UserId)
            }
            orderedBusinesses[i] = *b
        }
        if user.ActiveBusinessId != activeBusiness.BusinessId {
            l.log.Errorf("Active business '%s' does not match business '%s' for user '%s'", user.ActiveBusinessId, activeBusiness.BusinessId, user.UserId)
            metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler, 1)
        }
        return l.showAiReplySettingsForMultiBusiness(replyToken, user, orderedBusinesses, activeBusiness.BusinessId)
    }

    return l.showAiReplySettingsForSingleBusiness(replyToken, user, activeBusiness)
}

func (l *Line) showAiReplySettingsForSingleBusiness(replyToken string, user model.User, business model.Business) error {
    flexMessage, err := l.buildAiReplySettingsFlexMessageForSingleBusiness(user, business)
    if err != nil {
        l.log.Error("Error building flex message in showAiReplySettingsForSingleBusiness: ", err)
    }

    var resp *linebot.BasicResponse
    if replyToken == util.TestReplyToken {
        resp, err = l.lineClient.PushMessage("Ucc29292b212e271132cee980c58e94eb", linebot.NewFlexMessage("AI 回覆設定", flexMessage)).Do()
    } else {
        resp, err = l.lineClient.ReplyMessage(replyToken, linebot.NewFlexMessage("AI 回覆設定", flexMessage)).Do()
    }
    if err != nil {
        l.log.Error("Error replying message in showAiReplySettingsForSingleBusiness: ", err)
        return err
    }

    l.log.Debugf("Successfully executed line.ReplyMessage in showAiReplySettingsForSingleBusiness to %s: %s", user.UserId, jsonUtil.AnyToJson(resp))

    return nil
}

func (l *Line) showAiReplySettingsForMultiBusiness(replyToken string, user model.User, orderedBusinesses []model.Business, activeBusinessId bid.BusinessId) error {
    flexMessage, err := l.buildAiReplySettingsFlexMessageForMultiBusiness(user, orderedBusinesses, activeBusinessId)
    if err != nil {
        l.log.Error("Error building flex message in buildAiReplySettingsFlexMessageForMultiBusiness: ", err)
    }

    var resp *linebot.BasicResponse
    if replyToken == util.TestReplyToken {
        resp, err = l.lineClient.PushMessage("Ucc29292b212e271132cee980c58e94eb", linebot.NewFlexMessage("AI 回覆設定", flexMessage)).Do()
    } else {
        resp, err = l.lineClient.ReplyMessage(replyToken, linebot.NewFlexMessage("AI 回覆設定", flexMessage)).Do()
    }
    if err != nil {
        l.log.Error("Error replying message in showAiReplySettingsForMultiBusiness: ", err)
        return err
    }

    l.log.Debugf("Successfully executed line.ReplyMessage in showAiReplySettingsForMultiBusiness to %s: %s", user.UserId, jsonUtil.AnyToJson(resp))

    return nil
}

func (l *Line) SendAiGeneratedReply(aiReply string, review model.Review, generateAuthorName string, business model.Business, user model.User, userDao *ddbDao.UserDao) error {
    // for each user of the business, retrieve businessId Index for the user, and send the message
    for _, userId := range business.UserIds {
        var businessIdIndex int
        var err error
        // user already retrieved
        if userId == user.UserId {
            businessIdIndex, err = user.GetBusinessIdIndex(business.BusinessId)
            if err != nil {
                l.log.Errorf("Error getting businessIdIndex for business '%s' in SendAiGeneratedReply: %v", business.BusinessId, err)
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler, 1)
                continue
            }
        } else {
            sendingUser, err := userDao.GetUser(userId)
            if err != nil {
                l.log.Errorf("Error getting user '%s' in SendAiGeneratedReply: %v", userId, err)
                return err
            }
            if sendingUser == nil {
                l.log.Errorf("User '%s' not found in SendAiGeneratedReply. Inconsistent userIds in business '%s'", userId, business.BusinessId)
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler, 1)
                continue
            }
            businessIdIndex, err = sendingUser.GetBusinessIdIndex(business.BusinessId)
            if err != nil {
                l.log.Errorf("Error getting businessIdIndex for business '%s' in user '%s' during SendAiGeneratedReply: %v", business.BusinessId, sendingUser.UserId, err)
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler, 1)
                continue
            }
        }
        flexMessage, err := l.buildAiGeneratedReplyFlexMessage(review, aiReply, generateAuthorName, business.BusinessId, businessIdIndex)
        if err != nil {
            l.log.Error("Error building flex message in SendAiGeneratedReply: ", err)
            metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler, 1)
            continue
        }

        _, err = l.lineClient.PushMessage(userId, linebot.NewFlexMessage("AI 回覆生成結果", flexMessage)).Do()
        if err != nil {
            l.log.Error("Error sending message in SendAiGeneratedReply: ", err)
            return err
        }

        l.log.Infof("Successfully executed LINE.PushMessage in SendAiGeneratedReply to %s", userId)
    }

    return nil
}

func (l *Line) SendAuthRequest(userId string) error {
    authRedirectUrl, err := awsUtil.NewAws(l.log).GetAuthRedirectUrl()
    if err != nil {
        l.log.Error("Error getting auth redirect url in ReplyAuthRequest: ", err)
        return err
    }

    flexMessage, err := l.buildAuthRequestFlexMessage(userId, authRedirectUrl)
    if err != nil {
        l.log.Error("Error building flex message in RequestAuth: ", err)
    }

    resp, err := l.lineClient.PushMessage(userId, linebot.NewFlexMessage("智引力請求訪問 Google 資料", flexMessage)).Do()
    if err != nil {
        l.log.Error("Error replying message in ReplyAuthRequest: ", err)
        return err
    }

    l.log.Infof("Successfully requested auth to user '%s': %s", userId, jsonUtil.AnyToJson(resp))

    return nil
}

func (l *Line) ReplyAuthRequest(replyToken string, userId string) error {
    authRedirectUrl, err := awsUtil.NewAws(l.log).GetAuthRedirectUrl()
    if err != nil {
        l.log.Error("Error getting auth redirect url in ReplyAuthRequest: ", err)
        return err
    }

    flexMessage, err := l.buildAuthRequestFlexMessage(userId, authRedirectUrl)
    if err != nil {
        l.log.Error("Error building flex message in RequestAuth: ", err)
    }

    resp, err := l.lineClient.ReplyMessage(replyToken, linebot.NewFlexMessage("智引力請求訪問 Google 資料", flexMessage)).Do()
    if err != nil {
        l.log.Error("Error replying message in ReplyAuthRequest: ", err)
        return err
    }

    l.log.Infof("Successfully requested auth to user '%s': %s", userId, jsonUtil.AnyToJson(resp))

    return nil
}

func (l *Line) ReplyUserReplyFailed(replyToken string, reviewerName string, isAutoReply bool) (*linebot.BasicResponse, error) {
    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(buildReplyFailedMessage(reviewerName, isAutoReply))).Do()
}

func (l *Line) NotifyUsersReplyFailed(userIds []string, reviewerName string, isAutoReply bool) error {
    returnErr := error(nil)
    for _, userId := range userIds {
        _, err := l.lineClient.PushMessage(userId, linebot.NewTextMessage(buildReplyFailedMessage(reviewerName, isAutoReply))).Do()
        if err != nil {
            l.log.Errorf("Error sending message to '%s' in NotifyUsersReplyFailed: %v", userId, err)
            returnErr = err
        }
    }
    return returnErr
}

// ReplyUserReplyFailedWithReason replies to the user that the reply failed with the reason
// both the reviewerName and reason can be empty
func (l *Line) ReplyUserReplyFailedWithReason(replyToken string, reviewerName string, reason string) (*linebot.BasicResponse, error) {
    var text string
    if util.IsEmptyString(reviewerName) {
        text = "回覆評論失敗。"
    } else {
        text = fmt.Sprintf("回覆 %s 的評論失敗。", reviewerName)
    }
    text += reason + "很抱歉為您造成不便。"

    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(text)).Do()
}

func buildReplyFailedMessage(reviewerName string, isAutoReply bool) string {
    if isAutoReply {
        return fmt.Sprintf("自動回覆 %s 的評論失敗。很抱歉為您造成不便。", reviewerName)
    } else {
        return fmt.Sprintf("回覆 %s 的評論失敗，請稍後再試。很抱歉為您造成不便。", reviewerName)
    }
}

// NotifyReviewAutoReplied notifies all users of the business that owns the review that the review has been replied to
// param review: the review that was replied to
// param reply: the reply to the review
// param business: the business that owns the review
// param userDao: the userDao
func (l *Line) NotifyReviewAutoReplied(
    review model.Review,
    reply string,
    business model.Business,
    userDao *ddbDao.UserDao,
) error {
    for _, userId := range business.UserIds {
        sendingUser, err := userDao.GetUser(userId)
        if err != nil {
            l.log.Errorf("Error getting user '%s' in NotifyReviewReplied: %v", userId, err)
            return err
        }
        if sendingUser == nil {
            l.log.Errorf("User '%s' not found in NotifyReviewReplied. Inconsistent userIds in business '%s'", userId, business.BusinessId)
            metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler, 1)
            continue
        }
        businessIdIndex, err := sendingUser.GetBusinessIdIndex(business.BusinessId)
        if err != nil {
            l.log.Errorf("Error getting businessIdIndex for business '%s' in user '%s' during NotifyReviewReplied: %v", business.BusinessId, sendingUser.UserId, err)
            metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler, 1)
            continue
        }

        flexMessage, err := l.buildReviewRepliedNotificationMessage(review, reply, "自動回覆", true, business.BusinessName, businessIdIndex)
        if err != nil {
            l.log.Error("Error building flex message in NotifyReviewReplied: ", err)
            return err
        }

        _, err = l.lineClient.PushMessage(userId, linebot.NewFlexMessage("評論回覆通知", flexMessage)).Do()
        if err != nil {
            l.log.Errorf("Error sending message to '%s' in NotifyReviewReplied: %v . Flex Message: %s", userId, err, jsonUtil.AnyToJson(flexMessage))
            metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler, 1)
            continue
        }

        l.log.Infof("Successfully executed line.PushMessage/ReplyMessage in NotifyReviewReplied to user '%s'", userId)
    }

    return nil
}

// NotifyReviewReplied notifies all users of the business that owns the review that the review has been replied to
// param replyToken: the reply token of the user who replied to the review
// param replyTokenOwnerUserId: the userId of the user who replied to the review
// param review: the review that was replied to
// param business: the business that owns the review
// param replierUser: the user who replied to the review
// param userDao: the userDao
func (l *Line) NotifyReviewReplied(
    replyToken string,
    review model.Review,
    reply string,
    business model.Business,
    replierUser model.User,
    userDao *ddbDao.UserDao,
) error {
    for _, userId := range business.UserIds {
        if !util.IsEmptyString(replyToken) && userId == replierUser.UserId && replyToken != util.TestReplyToken {
            l.log.Debugf("Sending reply message to reply token owner user '%s'", replierUser.UserId)
            businessIdIndex, err := replierUser.GetBusinessIdIndex(business.BusinessId)
            if err != nil {
                l.log.Errorf("Error getting businessIdIndex for business '%s' in user '%s' during NotifyReviewReplied: %v", business.BusinessId, replierUser.UserId, err)
                return err
            }
            flexMessage, err := l.buildReviewRepliedNotificationMessage(review, reply, replierUser.LineUsername, false, business.BusinessName, businessIdIndex)
            if err != nil {
                l.log.Error("Error building flex message in NotifyReviewReplied: ", err)
                return err
            }
            _, err = l.lineClient.ReplyMessage(replyToken, linebot.NewFlexMessage("評論回覆通知", flexMessage)).Do()
            if err != nil {
                l.log.Errorf("Error replying to '%s' in NotifyReviewReplied: %v . Flex Message: %s", userId, err, jsonUtil.AnyToJson(flexMessage))
            }
        } else {
            sendingUser, err := userDao.GetUser(userId)
            if err != nil {
                l.log.Errorf("Error getting user '%s' in NotifyReviewReplied: %v", userId, err)
                return err
            }
            if sendingUser == nil {
                l.log.Errorf("User '%s' not found in NotifyReviewReplied. Inconsistent userIds in business '%s'", userId, business.BusinessId)
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler, 1)
                continue
            }
            businessIdIndex, err := sendingUser.GetBusinessIdIndex(business.BusinessId)
            if err != nil {
                l.log.Errorf("Error getting businessIdIndex for business '%s' in user '%s' during NotifyReviewReplied: %v", business.BusinessId, sendingUser.UserId, err)
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler, 1)
                continue
            }

            flexMessage, err := l.buildReviewRepliedNotificationMessage(review, reply, replierUser.LineUsername, false, business.BusinessName, businessIdIndex)
            if err != nil {
                l.log.Error("Error building flex message in NotifyReviewReplied: ", err)
                return err
            }

            _, err = l.lineClient.PushMessage(userId, linebot.NewFlexMessage("評論回覆通知", flexMessage)).Do()
            if err != nil {
                l.log.Errorf("Error sending message to '%s' in NotifyReviewReplied: %v . Flex Message: %s", userId, err, jsonUtil.AnyToJson(flexMessage))
                metric.EmitLambdaMetric(enum.Metric5xxError, enum2.HandlerNameLineEventsHandler, 1)
                continue
            }
        }

        l.log.Infof("Successfully executed line.PushMessage/ReplyMessage in NotifyReviewReplied to user '%s'", userId)
    }

    return nil
}

// NotifyUserUpdateFailed let user know that the update failed
// param updateType: is the Mandarin text of the update type in notification
// Example:快速回覆訊息, 關鍵字, 主要業務
func (l *Line) NotifyUserUpdateFailed(replyToken string, updateType string) (*linebot.BasicResponse, error) {
    text := fmt.Sprintf("%s更新失敗，請稍後再試。很抱歉為您造成不便。", updateType)

    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(text)).Do()
}

func (l *Line) NotifyUserAiReplyGenerationInProgress(replyToken string) (*linebot.BasicResponse, error) {
    text := fmt.Sprintf("AI 回覆生成中…")

    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(text)).Do()
}

func (l *Line) NotifyUserAiReplyGenerationFailed(userId string) (*linebot.BasicResponse, error) {
    text := fmt.Sprintf("AI 回覆生成失敗，請稍後再試。很抱歉為您造成不便。")

    return l.lineClient.PushMessage(userId, linebot.NewTextMessage(text)).Do()
}

func (l *Line) ReplyHelpMessage(replyToken string) (*linebot.BasicResponse, error) {
    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(util.HelpMessage())).Do()
}

func (l *Line) ReplyMoreMessage(replyToken string) (*linebot.BasicResponse, error) {
    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(util.MoreMessage())).Do()
}

func (l *Line) ReplyUser(replyToken string, message string) (*linebot.BasicResponse, error) {
    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(message)).Do()
}

func (l *Line) NotifyUserCannotUseLineEmoji(replyToken string) (*linebot.BasicResponse, error) {
    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(CannotUseLineEmojiMessage)).Do()
}

func (l *Line) ParseRequest(request *events.LambdaFunctionURLRequest) ([]*linebot.Event, error) {
    httpRequest := convertToHttpRequest(request)
    return l.lineClient.ParseRequest(httpRequest)
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

func newLineClient(log *zap.SugaredLogger) *linebot.Client {
    secrets := awsUtil.NewAws(log).GetSecrets()
    lineClient, err := linebot.New(secrets.LineChannelSecret, secrets.LineChannelAccessToken)
    if err != nil {
        log.Fatal("cannot create new Line Client", err)
    }

    return lineClient
}

func (l *Line) NotifyQuickReplySettingsUpdated(userIds []string, updaterName string, businessName string) error {
    flexMessage, err := l.buildQuickReplySettingsUpdatedNotificationMessage(updaterName, businessName)
    if err != nil {
        l.log.Error("Error building flex message in NotifyQuickReplySettingsUpdated: ", err)
        return err
    }

    for _, userId := range userIds {
        _, err = l.lineClient.PushMessage(userId, linebot.NewFlexMessage("快速回覆設定更新通知", flexMessage)).Do()
        if err != nil {
            l.log.Errorf("Error sending message to '%s' in NotifyQuickReplySettingsUpdated: %v", userId, err)
        } else {
            l.log.Infof("Successfully executed line.PushMessage in NotifyQuickReplySettingsUpdated to user '%s'", userId)
        }
    }

    return err
}

func (l *Line) NotifyAiReplySettingsUpdated(userIds []string, updaterName string, businessName string) error {
    flexMessage, err := l.buildAiReplySettingsUpdatedNotificationMessage(updaterName, businessName)
    if err != nil {
        l.log.Error("Error building flex message in NotifyAiReplySettingsUpdated: ", err)
        return err
    }

    for _, userId := range userIds {
        _, err = l.lineClient.PushMessage(userId, linebot.NewFlexMessage("AI回覆設定更新通知", flexMessage)).Do()
        if err != nil {
            l.log.Errorf("Error sending message to '%s' in NotifyAiReplySettingsUpdated: %v", userId, err)
        } else {
            l.log.Infof("Successfully executed line.PushMessage in NotifyAiReplySettingsUpdated to user '%s'", userId)
        }
    }

    return err
}
