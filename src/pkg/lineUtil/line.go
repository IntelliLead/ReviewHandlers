package lineUtil

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/awsUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
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
}

func NewLine(logger *zap.SugaredLogger) *Line {
    return &Line{
        lineClient:         newLineClient(logger),
        log:                logger,
        reviewMessageJsons: jsonUtil.LoadReviewMessageLineFlexTemplateJsons(),
        quickReplyJsons:    jsonUtil.LoadQuickReplySettingsLineFlexTemplateJsons(),
        aiReplyJsons:       jsonUtil.LoadAiReplyLineFlexTemplateJsons(),
        authJsons:          jsonUtil.LoadAuthLineFlexTemplateJsons(),
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

func (l *Line) SendNewReview(review model.Review, user model.User) error {
    flexMessage, err := l.buildReviewFlexMessage(review, user)
    if err != nil {
        l.log.Error("Error building flex message in SendNewReview: ", err)
    }

    resp, err := l.lineClient.PushMessage(review.UserId, linebot.NewFlexMessage("您有新的Google Map 評論！", flexMessage)).Do()
    if err != nil {
        l.log.Error("Error sending lineTextMessage to line in SendNewReview: ", err)
        return err
    }

    l.log.Debugf("Successfully executed line.PushMessage in SendNewReview to %s: %s", review.UserId, jsonUtil.AnyToJson(resp))

    return nil
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

func (l *Line) SendAiGeneratedReply(aiReply string, review model.Review) error {
    flexMessage, err := l.buildAiGeneratedReplyFlexMessage(review, aiReply)
    if err != nil {
        l.log.Error("Error building flex message in SendAiGeneratedReply: ", err)
    }

    resp, err := l.lineClient.PushMessage(review.UserId, linebot.NewFlexMessage("AI 回覆生成結果", flexMessage)).Do()
    if err != nil {
        l.log.Error("Error sending message in SendAiGeneratedReply: ", err)
        return err
    }

    l.log.Debugf("Successfully executed PushMessage in SendAiGeneratedReply to %s: %s", review.UserId, jsonUtil.AnyToJson(resp))

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

func (l *Line) RequestAuth(userId string, authRedirectUrl string) error {
    flexMessage, err := l.buildAuthRequestFlexMessage(userId, authRedirectUrl)
    if err != nil {
        l.log.Error("Error building flex message in RequestAuth: ", err)
    }

    resp, err := l.lineClient.PushMessage(userId, linebot.NewFlexMessage("智引力請求訪問 Google 資料", flexMessage)).Do()
    if err != nil {
        l.log.Error("Error sending message in RequestAuth: ", err)
        return err
    }

    l.log.Infof("Successfully requested auth to user '%s': %s", userId, jsonUtil.AnyToJson(resp))

    return nil
}

func (l *Line) ReplyUserReplyProcessed(replyToken string, succeeded bool, reviewerName string, isAutoReply bool) (*linebot.BasicResponse, error) {
    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(buildReplyProcessedMessage(succeeded, reviewerName, isAutoReply))).Do()
}

func (l *Line) NotifyUserReplyProcessed(userId string, succeeded bool, reviewerName string, isAutoReply bool) (*linebot.BasicResponse, error) {
    return l.lineClient.PushMessage(userId, linebot.NewTextMessage(buildReplyProcessedMessage(succeeded, reviewerName, isAutoReply))).Do()
}

func buildReplyProcessedMessage(succeeded bool, reviewerName string, isAutoReply bool) string {
    var text string
    if succeeded {
        if isAutoReply {
            text = fmt.Sprintf("已使用快速回覆內容自動回覆 %s 的評論。感謝您使用智引力。", reviewerName)
        } else {
            text = fmt.Sprintf("已回覆 %s 的評論。感謝您使用智引力。", reviewerName)
        }
    } else {
        if isAutoReply {
            text = fmt.Sprintf("自動回覆 %s 的評論失敗。很抱歉為您造成不便。", reviewerName)
        } else {
            text = fmt.Sprintf("回覆 %s 的評論失敗，請稍後再試。很抱歉為您造成不便。", reviewerName)
        }
    }

    return text
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

func (l *Line) ReplyUserReviewReplyProcessedWithReason(replyToken string, succeeded bool, reviewerName string, reason string) (*linebot.BasicResponse, error) {
    var text string
    if succeeded {
        text = fmt.Sprintf("已回覆 %s 的評論。感謝您使用智引力。", reviewerName)
    } else {
        text = fmt.Sprintf("回覆 %s 的評論失敗。%s", reviewerName, reason)
    }

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
