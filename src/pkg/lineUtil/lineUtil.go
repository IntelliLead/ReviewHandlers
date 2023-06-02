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

type LineUtil struct {
    lineClient *linebot.Client
    logger     *zap.SugaredLogger
}

func NewLineUtil(logger *zap.SugaredLogger) *LineUtil {
    return &LineUtil{
        lineClient: newLineClient(logger),
        logger:     logger,
    }
}

func (l *LineUtil) SendNewReview(review model.Review) error {
    readableReviewTimestamp, err := util.UtcToReadableTwTimestamp(review.ReviewLastUpdated)
    if err != nil {
        l.logger.Error("Error converting review timestamp to readable format: ", err)
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
        l.logger.Error("Error sending message to line: ", err)
        return err
    }

    l.logger.Debugf("Successfully executed line.PushMessage in SendNewReview to %s: %s", review.UserId, util.AnyToJson(resp))
    return nil
}

func (l *LineUtil) SendQuickReply(replyToken string) (*linebot.BasicResponse, error) {
    message := linebot.NewTextMessage("").WithQuickReplies(linebot.NewQuickReplyItems(
        // label` must not be longer than 20 characters
        linebot.NewQuickReplyButton(
            "",
            linebot.NewMessageAction("NewMsgActionLabel", "NewMessageActionText"),
        ),
        linebot.NewQuickReplyButton(
            "",
            linebot.NewURIAction("NewURIActionLabel", "https://google.com"),
        ),
        linebot.NewQuickReplyButton(
            "",
            linebot.NewPostbackAction("NewPostBkActionLabel", "action=buy&itemid=111", "", "displayText", linebot.InputOptionOpenKeyboard, "---\nName: \nPhone: \nBirthday: \n---"),
        ),
    ))

    return l.lineClient.ReplyMessage(replyToken, message).Do()
}

func (l *LineUtil) ParseRequest(request *events.LambdaFunctionURLRequest) ([]*linebot.Event, error) {
    httpRequest := convertToHttpRequest(request)
    l.logger.Debug("wrapped HTTP request is: ", request)

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

func newLineClient(logger *zap.SugaredLogger) *linebot.Client {
    // Create a new LINE Bot client
    channelSecret := "a6064245795375fee1fb9cc2e4711447"
    channelAccessToken := "0PWI55x6HFQ1WfHOBTddspgVTpTbFtFmy9ImN7NuYqScSz0mTFjYDqb9dA8TeRaUHNCrAWJ0x6yv4iJiMNrki4ZuYS4UhntFFtKma5tocBpgMcnD8+Kg0cTz3yoghq24QKmKp7R7OfoaTn4i/m7Y1AdB04t89/1O/w1cDnyilFU="
    lineClient, err := linebot.New(channelSecret, channelAccessToken)
    if err != nil {
        logger.Fatal("cannot create new Line Client", err)
    }

    return lineClient
}
