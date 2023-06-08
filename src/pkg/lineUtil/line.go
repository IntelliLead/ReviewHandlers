package lineUtil

import (
    "fmt"
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
    lineClient *linebot.Client
    log        *zap.SugaredLogger
}

func NewLine(logger *zap.SugaredLogger) *Line {
    return &Line{
        lineClient: newLineClient(logger),
        log:        logger,
    }
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

    l.log.Debugf("Successfully executed line.ReplyMessage in SendUnknownResponseReply: %s", util.AnyToJson(resp))
    return nil
}

func (l *Line) SendNewReview(review model.Review) error {
    readableReviewTimestamp, err := util.UtcToReadableTwTimestamp(review.ReviewLastUpdated)
    if err != nil {
        l.log.Error("Error converting review timestamp to readable format: ", err)
        return err
    }
    // send message with quick reply options
    reviewMessage := fmt.Sprintf("$ 您有新評論 @%s ！\n\n評論內容：\n%s\n\n評價：%s\n評論者：%s\n評論時間：%s\n",
        review.ReviewId.String(), review.Review, review.NumberRating.String(), review.ReviewerName, readableReviewTimestamp)

    message := linebot.NewTextMessage(reviewMessage).WithQuickReplies(linebot.NewQuickReplyItems(
        // label` must not be longer than 20 characters
        linebot.NewQuickReplyButton(
            "",
            linebot.NewPostbackAction("快速回復", "any", "", "", linebot.InputOptionOpenKeyboard, fmt.Sprintf("@%s 感謝…", review.ReviewId.String())),
        ),
    ))
    message.AddEmoji(linebot.NewEmoji(0, "5ac2280f031a6752fb806d65", "001"))

    resp, err := l.lineClient.PushMessage(review.UserId, message).Do()
    if err != nil {
        l.log.Error("Error sending message to line: ", err)
        return err
    }

    l.log.Debugf("Successfully executed line.PushMessage in SendNewReview to %s: %s", review.UserId, util.AnyToJson(resp))
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

func (l *Line) ReplyHelpMessage(replyToken string) (*linebot.BasicResponse, error) {
    text := fmt.Sprint("本服務目前僅用於回復Google Maps 評論。\n" +
        "回復最新評論：使用評論訊息下方\"快速回復\"按鈕即可編輯回復內容。\n\n" +
        "若需回復非最新評論：評論皆有編號，請在回復時以 @編號 作為開頭。例如，如果評論編號為\"@8F\"，則回復\"@8F 感謝您的認可！\"\n\n" +
        "若需更新回復內容：以 @編號 作為開頭照常回復即可。\n\n" +
        "新評論2分鐘內會推送到這裡。新星評（無評價內容）不會被推送。\n" +
        "評論者更新自己的已留評論不會被推送。\n\n" +
        "如需更多幫助，請聯係我們：")
    text = text + "https://line.me/R/ti/p/%40006xnyvp"

    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(text)).Do()

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
    // Create a new LINE Bot client
    channelSecret := "aa8c492c6295d7e3857fca4b41f49604"
    channelAccessToken := "AqTNC1x18DT0/e1rkVUEnigmwyyHj4cPa+TbX1ECE5NVfzeB7OPLUsQjRkXrbCzBp7etk9Skni4/8NZW9dBR6eDbeKTA+4CNFOtHEF5sHp+1nXDJ2dzQnuf/NV0vuqMju7iznWvpLaSGKbRonLs6FgdB04t89/1O/w1cDnyilFU="
    lineClient, err := linebot.New(channelSecret, channelAccessToken)
    if err != nil {
        log.Fatal("cannot create new Line Client", err)
    }

    return lineClient
}
