package lineUtil

import (
    "encoding/json"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "strings"
    "unicode"
)

func IsReviewReplyMessage(message string) bool {
    return strings.HasPrefix(message, "@")
}

func ParseReplyMessage(str string) (model.ReplyMessage, error) {
    if !strings.HasPrefix(str, "@") {
        return model.ReplyMessage{}, fmt.Errorf("message is not a reply message: %s", str)
    }

    // Find the first whitespace character after '@'
    index := strings.IndexFunc(str[1:], isWhitespace)
    if index == -1 {
        return model.NewReplyMessage(_type.NewReviewId(str[1:]), "") // Return the remaining text after '@' as ReviewId
    }

    reviewID := str[1 : index+1]
    replyMsg := strings.TrimSpace(str[index+2:])

    return model.NewReplyMessage(_type.NewReviewId(reviewID), replyMsg)
}

func isWhitespace(r rune) bool {
    return unicode.IsSpace(r)
}

func getMessageType(event *linebot.Event) (linebot.MessageType, error) {
    // LINE Go SDK is bugged, this is the workaround
    // log.Debug("message type is ", event.Message.Type)        // message type is 0xa13460
    // log.Debug("message type() is ", event.Message.Type())    // message type() is

    jsonObj := util.AnyToJsonObject(event)

    // Define a struct to hold the JSON object
    var data struct {
        Message message `json:"message"`
    }
    // Unmarshal the JSON data into the struct
    err := json.Unmarshal(jsonObj, &data)
    if err != nil {
        return "", err
    }
    // Access the value of message.type
    return data.Message.Type, nil
}

func IsTextMessageFromUser(event *linebot.Event) (bool, error) {
    if event.Type != linebot.EventTypeMessage {
        // not even message event
        return false, nil
    }

    messageType, err := getMessageType(event)
    if err != nil {
        return false, err
    }
    return event.Source.Type == linebot.EventSourceTypeUser && messageType == linebot.MessageTypeText, nil
}

type message struct {
    Type linebot.MessageType `json:"type"`
}
