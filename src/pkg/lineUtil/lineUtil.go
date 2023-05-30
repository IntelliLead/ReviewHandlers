package lineUtil

import (
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

func NewLineUtil(client *linebot.Client, logger *zap.SugaredLogger) *LineUtil {
	return &LineUtil{
		lineClient: client,
		logger:     logger,
	}
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
