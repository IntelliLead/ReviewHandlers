package lineUtil

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/awsUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
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
func (l *Line) SendNewReview(review model.Review, business model.Business) error {
    quickReplyMessage := ""
    if !util.IsEmptyStringPtr(business.QuickReplyMessage) {
        quickReplyMessage = business.GetFinalQuickReplyMessage(review)
    }

    flexMessage, err := l.buildReviewFlexMessage(review, quickReplyMessage)
    if err != nil {
        l.log.Error("Error building flex message in SendNewReview: ", err)
    }

    for _, userId := range business.UserIds {
        _, err := l.lineClient.PushMessage(userId, linebot.NewFlexMessage("您有新的Google Map 評論！", flexMessage)).Do()
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
func (l *Line) SendNewReviewToUser(review model.Review, user model.User) error {
    quickReplyMessage := ""
    if !util.IsEmptyStringPtr(user.QuickReplyMessage) {
        quickReplyMessage = user.GetFinalQuickReplyMessage(review)
    }

    flexMessage, err := l.buildReviewFlexMessage(review, quickReplyMessage)
    if err != nil {
        l.log.Error("Error building flex message in SendNewReview: ", err)
    }

    _, err = l.lineClient.PushMessage(user.UserId, linebot.NewFlexMessage("您有新的Google Map 評論！", flexMessage)).Do()
    if err != nil {
        l.log.Errorf("Error sending lineTextMessage to LINE user %s in SendNewReview: %v", user.UserId, err)
        return err
    }

    return nil
}

func (l *Line) ShowQuickReplySettingsByBusinessGet(replyToken string, businessId bid.BusinessId, businessDao *ddbDao.BusinessDao) error {
    businessPtr, err := businessDao.GetBusiness(businessId)
    if err != nil {
        l.log.Error("Error getting business in ShowQuickReplySettings: ", err)
        return err
    }
    if businessPtr == nil {
        l.log.Errorf("Business '%s' not found", businessId)
        return errors.New(fmt.Sprintf("Business '%s' not found", businessId))
    }
    business := *businessPtr

    return l.ShowQuickReplySettings(replyToken, business.AutoQuickReplyEnabled, business.QuickReplyMessage)
}

func (l *Line) ShowQuickReplySettings(replyToken string, autoQuickReplyEnabled bool, quickReplyMessage *string) error {
    flexMessage, err := l.buildQuickReplySettingsFlexMessage(autoQuickReplyEnabled, quickReplyMessage)
    if err != nil {
        l.log.Error("Error building flex message in ShowQuickReplySettings: ", err)
    }

    resp, err := l.lineClient.ReplyMessage(replyToken, linebot.NewFlexMessage("設定快速回覆", flexMessage)).Do()
    if err != nil {
        l.log.Error("Error replying message in ShowQuickReplySettings: ", err)
        return err
    }

    l.log.Debugf("Successfully executed line.ReplyMessage in ShowQuickReplySettings: %s", jsonUtil.AnyToJson(resp))

    return nil
}

func (l *Line) ShowAiReplySettingsByBusinessGet(replyToken string, user model.User, businessDao *ddbDao.BusinessDao) error {
    businessId := user.ActiveBusinessId
    businessPtr, err := businessDao.GetBusiness(businessId)
    if err != nil {
        l.log.Error("Error getting business in ShowAiReplySettingsByBusinessGet: ", err)
        return err
    }
    if businessPtr == nil {
        l.log.Errorf("Business '%s' not found", businessId)
        return errors.New(fmt.Sprintf("Business '%s' not found", businessId))
    }
    business := *businessPtr

    return l.ShowAiReplySettings(replyToken, user, business)
}

func (l *Line) ShowAiReplySettings(replyToken string, user model.User, business model.Business) error {
    flexMessage, err := l.buildAiReplySettingsFlexMessage(user, business)
    if err != nil {
        l.log.Error("Error building flex message in ShowAiReplySettings: ", err)
    }

    resp, err := l.lineClient.ReplyMessage(replyToken, linebot.NewFlexMessage("AI 回覆設定", flexMessage)).Do()
    if err != nil {
        l.log.Error("Error replying message in ShowAiReplySettings: ", err)
        return err
    }

    l.log.Debugf("Successfully executed line.ReplyMessage in ShowAiReplySettings to %s: %s", user.UserId, jsonUtil.AnyToJson(resp))

    return nil
}

func (l *Line) SendAiGeneratedReply(aiReply string, review model.Review, userIds []string, generateAuthorName string) error {
    flexMessage, err := l.buildAiGeneratedReplyFlexMessage(review, aiReply, generateAuthorName)
    if err != nil {
        l.log.Error("Error building flex message in SendAiGeneratedReply: ", err)
    }

    for _, userId := range userIds {
        _, err := l.lineClient.PushMessage(userId, linebot.NewFlexMessage("AI 回覆生成結果", flexMessage)).Do()
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

func (l *Line) NotifyUserReplyFailed(userId string, reviewerName string, isAutoReply bool) (*linebot.BasicResponse, error) {
    return l.lineClient.PushMessage(userId, linebot.NewTextMessage(buildReplyFailedMessage(reviewerName, isAutoReply))).Do()
}

func buildReplyFailedMessage(reviewerName string, isAutoReply bool) string {
    if isAutoReply {
        return fmt.Sprintf("自動回覆 %s 的評論失敗。很抱歉為您造成不便。", reviewerName)
    } else {
        return fmt.Sprintf("回覆 %s 的評論失敗，請稍後再試。很抱歉為您造成不便。", reviewerName)
    }
}

func (l *Line) NotifyReviewReplied(
    userIds []string,
    replyToken *string,
    replyTokenOwnerUserId *string,
    review model.Review,
    reply string,
    replierName string,
    isAutoReply bool,
    businessName *string) error {
    flexMessage, err := l.buildReviewRepliedNotificationMessage(review, reply, replierName, isAutoReply, businessName)
    if err != nil {
        l.log.Error("Error building flex message in NotifyReviewReplied: ", err)
        return err
    }

    err = nil
    for _, userId := range userIds {
        if !util.IsEmptyStringPtr(replyToken) && !util.IsEmptyStringPtr(replyTokenOwnerUserId) && userId == *replyTokenOwnerUserId && *replyToken != util.TestReplyToken {
            l.log.Debugf("Sending reply message to reply token owner user '%s'", *replyTokenOwnerUserId)
            _, err = l.lineClient.ReplyMessage(*replyToken, linebot.NewFlexMessage("評論回覆通知", flexMessage)).Do()
        } else {
            _, err = l.lineClient.PushMessage(userId, linebot.NewFlexMessage("評論回覆通知", flexMessage)).Do()
        }
        if err != nil {
            l.log.Errorf("Error sending message to '%s' in NotifyReviewReplied: %v . Flex Message: %s", userId, err, jsonUtil.AnyToJson(flexMessage))
        } else {
            l.log.Infof("Successfully executed line.PushMessage/ReplyMessage in NotifyReviewReplied to user '%s'", userId)
        }
    }

    return err
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

func (l *Line) ReplyUserReviewReplyFailedWithReason(replyToken string, reviewerName string, reason string) (*linebot.BasicResponse, error) {
    var text string
    text = fmt.Sprintf("回覆 %s 的評論失敗。%s", reviewerName, reason)

    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(text)).Do()
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

func (l *Line) NotifyQuickReplySettingsUpdated(userIds []string, updaterName string) error {
    flexMessage, err := l.buildQuickReplySettingsUpdatedNotificationMessage(updaterName)
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

func (l *Line) NotifyAiReplySettingsUpdated(userIds []string, updaterName string) error {
    flexMessage, err := l.buildAiReplySettingsUpdatedNotificationMessage(updaterName)
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
