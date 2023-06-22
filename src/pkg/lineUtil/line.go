package lineUtil

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/secret"
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
    aiReplyResultJsons jsonUtil.AiReplyResultLineFlexTemplateJsons
}

func NewLine(logger *zap.SugaredLogger) *Line {
    return &Line{
        lineClient:         newLineClient(logger),
        log:                logger,
        reviewMessageJsons: jsonUtil.LoadReviewMessageLineFlexTemplateJsons(),
        quickReplyJsons:    jsonUtil.LoadQuickReplySettingsLineFlexTemplateJsons(),
        aiReplyResultJsons: jsonUtil.LoadAiReplyResultLineFlexTemplateJsons(),
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

func (l *Line) SendUnknownResponseReply(replyToken string) error {
    reviewMessage := fmt.Sprintf("對不起，我還不會處理您的訊息。如需幫助，請回復\"/help\"")

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

    l.log.Debugf("Successfully executed line.ReplyMessage in SendUnknownResponseReply: %s", jsonUtil.AnyToJson(resp))
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

func (l *Line) ShowQuickReplySettings(replyToken string, user model.User, isUpdated bool) error {
    flexMessage, err := l.buildQuickReplySettingsFlexMessage(user, isUpdated)

    if err != nil {
        l.log.Error("Error building flex message in ShowQuickReplySettings: ", err)
    }

    resp, err := l.lineClient.ReplyMessage(replyToken, linebot.NewFlexMessage("設定快速回復", flexMessage)).Do()
    if err != nil {
        l.log.Error("Error replying message in ShowQuickReplySettings: ", err)
        return err
    }

    l.log.Debugf("Successfully executed line.ReplyMessage in ShowQuickReplySettings to %s: %s", user.UserId, jsonUtil.AnyToJson(resp))

    return nil
}

func (l *Line) SendAiGeneratedReply(replyToken string, aiReply string, review model.Review, userId string) error {
    flexMessage, err := l.buildAiGeneratedReplyFlexMessage(review, aiReply)
    if err != nil {
        l.log.Error("Error building flex message in SendAiGeneratedReply: ", err)
    }

    resp, err := l.lineClient.ReplyMessage(replyToken, linebot.NewFlexMessage("AI 回復生成結果", flexMessage)).Do()
    if err != nil {
        l.log.Error("Error replying message in SendAiGeneratedReply: ", err)
        return err
    }

    l.log.Debugf("Successfully executed line.ReplyMessage in SendAiGeneratedReply to %s: %s", userId, jsonUtil.AnyToJson(resp))

    return nil
}

func (l *Line) NotifyUserReplyProcessed(replyToken string, succeeded bool, reviewerName string) (*linebot.BasicResponse, error) {
    var text string
    if succeeded {
        text = fmt.Sprintf("已回復 %s 的評論。感謝您使用智引力。", reviewerName)
    } else {
        text = fmt.Sprintf("回復 %s 的評論失敗，請稍後再試。很抱歉為您造成不便。", reviewerName)
    }

    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(text)).Do()
}

func (l *Line) NotifyUserUpdateQuickReplyMessageFailed(replyToken string) (*linebot.BasicResponse, error) {
    text := fmt.Sprintf("快速回復訊息更新失敗，請稍後再試。很抱歉為您造成不便。")

    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(text)).Do()
}

func (l *Line) ReplyHelpMessage(replyToken string) (*linebot.BasicResponse, error) {
    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(util.HelpMessage())).Do()
}

func (l *Line) ReplyMoreMessage(replyToken string) (*linebot.BasicResponse, error) {
    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(util.MoreMessage())).Do()
}

func (l *Line) NotifyUserReplyProcessedWithReason(replyToken string, succeeded bool, reviewerName string, reason string) (*linebot.BasicResponse, error) {
    var text string
    if succeeded {
        text = fmt.Sprintf("已回復 %s 的評論。感謝您使用智引力。", reviewerName)
    } else {
        text = fmt.Sprintf("回復 %s 的評論失敗。%s", reviewerName, reason)
    }

    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(text)).Do()
}

func (l *Line) ParseRequest(request *events.LambdaFunctionURLRequest) ([]*linebot.Event, error) {
    httpRequest := convertToHttpRequest(request)
    l.log.Debug("wrapped HTTP request is: ", request)

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
    secrets := secret.GetSecrets()
    lineClient, err := linebot.New(secrets.LineChannelSecret, secrets.LineChannelAccessToken)
    if err != nil {
        log.Fatal("cannot create new Line Client", err)
    }

    return lineClient
}
