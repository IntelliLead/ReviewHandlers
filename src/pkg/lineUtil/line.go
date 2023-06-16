package lineUtil

import (
    "encoding/json"
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
    lineClient *linebot.Client
    log        *zap.SugaredLogger
    jsons      jsonUtil.LineFlexTemplateJsons
}

func NewLine(logger *zap.SugaredLogger) *Line {
    return &Line{
        lineClient: newLineClient(logger),
        log:        logger,
        jsons:      jsonUtil.LoadLineFlexTemplateJsons(),
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

func (l *Line) SendNewReview(review model.Review) error {
    // compose flex message
    // Convert the original JSON to a map[string]interface{}
    reviewMsgJson, err := jsonUtil.JsonToMap(l.jsons.ReviewMessage)
    if err != nil {
        l.log.Debug("Error unmarshalling reviewMessage JSON: ", err)
    }

    // update review message
    isEmptyReview := review.Review == ""

    var reviewMessage string
    if isEmptyReview {
        reviewMessage = "（星評無內容）"
    } else {
        reviewMessage = review.Review
    }

    if contents, ok := reviewMsgJson["body"].(map[string]interface{})["contents"]; ok {
        if contentsArr, ok := contents.([]interface{}); ok {
            contentsArr[3].(map[string]interface{})["text"] = reviewMessage
        }
    }

    // update stars
    starRatingJsonArr, err := review.NumberRating.LineFlexTemplateJson()
    if err != nil {
        l.log.Error("Error creating starRating JSON: ", err)
        return err
    }

    if contents, ok := reviewMsgJson["body"].(map[string]interface{})["contents"]; ok {
        if contentsArr, ok := contents.([]interface{}); ok {
            contentsArr[1].(map[string]interface{})["contents"] = starRatingJsonArr
        }
    }

    // update review time
    readableReviewTimestamp, err := util.UtcToReadableTwTimestamp(review.ReviewLastUpdated)
    if err != nil {
        l.log.Error("Error converting review timestamp to readable format: ", err)
        return err
    }

    // Modify the desired key in the map
    if contents, ok := reviewMsgJson["body"].(map[string]interface{})["contents"]; ok {
        if contentsArr, ok := contents.([]interface{}); ok {
            if subContents, ok := contentsArr[2].(map[string]interface{})["contents"]; ok {
                if subContentsArr, ok := subContents.([]interface{}); ok {
                    // reviewer and timestamp subtext level

                    // modify review timestamp
                    if subSubContents, ok := subContentsArr[0].(map[string]interface{})["contents"]; ok {
                        if subSubContentsArr, ok := subSubContents.([]interface{}); ok {
                            subSubContentsArr[1].(map[string]interface{})["text"] = readableReviewTimestamp
                        }
                    }

                    // modify reviewer
                    if subSubContents, ok := subContentsArr[1].(map[string]interface{})["contents"]; ok {
                        if subSubContentsArr, ok := subSubContents.([]interface{}); ok {
                            subSubContentsArr[1].(map[string]interface{})["text"] = review.ReviewerName
                        }
                    }

                }
            }
        }
    }

    // update edit reply button
    fillInText := fmt.Sprintf("@%s 感謝…", review.ReviewId.String())
    if contents, ok := reviewMsgJson["footer"].(map[string]interface{})["contents"]; ok {
        if contentsArr, ok := contents.([]interface{}); ok {
            if action, ok := contentsArr[1].(map[string]interface{})["action"]; ok {
                action.(map[string]interface{})["fillInText"] = fillInText
            }
        }
    }

    // update quick reply button
    // TODO: enable quick reply button. It is disabled for now because quick reply is not implemented
    if contents, ok := reviewMsgJson["footer"].(map[string]interface{})["contents"]; ok {
        if contentsArr, ok := contents.([]interface{}); ok {
            // skip 1st element
            reviewMsgJson["footer"].(map[string]interface{})["contents"] = append(contentsArr[1:])
        }
    }

    // Convert the map to LINE flex message
    // first convert back to json
    reviewMsgJsonBytes, err := json.Marshal(reviewMsgJson)
    if err != nil {
        l.log.Error("Error marshalling reviewMessage JSON: ", err)
        return err
    }
    reviewMsgFlexContainer, err := linebot.UnmarshalFlexMessageJSON(reviewMsgJsonBytes)
    if err != nil {
        l.log.Error("Error occurred during linebot.UnmarshalFlexMessageJSON: ", err)
        return err
    }

    resp, err := l.lineClient.PushMessage(review.UserId, linebot.NewFlexMessage("您有 Google Map 新評價", reviewMsgFlexContainer)).Do()
    if err != nil {
        l.log.Error("Error sending lineTextMessage to line: ", err)
        return err
    }

    l.log.Debugf("Successfully executed line.PushMessage in SendNewReview to %s: %s", review.UserId, jsonUtil.AnyToJson(resp))

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
    return l.lineClient.ReplyMessage(replyToken, linebot.NewTextMessage(util.HelpMessage())).Do()
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
