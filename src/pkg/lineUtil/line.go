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

func (l *Line) SendNewReview(review model.Review) error {
    readableReviewTimestamp, err := util.UtcToReadableTwTimestamp(review.ReviewLastUpdated)
    if err != nil {
        l.log.Error("Error converting review timestamp to readable format: ", err)
        return err
    }
    // send message with quick reply options
    reviewMessage := fmt.Sprintf("$ 您有新評論 @%s ！\n\n評論內容：\n%s\n評論者：%s\n評論時間：%s\n",
        review.ReviewId.String(), review.Review, review.ReviewerName, readableReviewTimestamp)

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
        text = fmt.Sprintf("已回復 %s 的評論。感謝使用智引力！", reviewerName)
    } else {
        text = fmt.Sprintf("回復 %s 的評論失敗，請稍後再試。抱歉為您造成不便。", reviewerName)
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
