package lineUtil

import (
    "encoding/json"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
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

func IsHelpMessage(message string) bool {
    // TODO: [INT-49] fix /h not working
    return isCommand(message, "/help") || isCommand(message, "/h ") || isCommand(message, "/幫助")
}

func IsUpdateQuickReplyMessage(message string) bool {
    return isCommand(message, "/QuickReply")
}

func isCommand(s string, cmd string) bool {
    if strings.HasPrefix(s, cmd) {
        remaining := strings.TrimPrefix(s, cmd)

        if remaining == "" || strings.TrimSpace(remaining) == "" || strings.HasPrefix(remaining, " ") {
            return true
        }
    }

    return false
}

// CommandMessage format: "/<command> <args>"
// e.g., "/Help xxx"
type CommandMessage struct {
    Command string
    Args    []string
}

func ParseCommandMessage(str string, isMultiArgs bool) CommandMessage {
    if !strings.HasPrefix(str, "/") {
        return CommandMessage{}
    }

    // Find the first whitespace character after '/'
    index := strings.IndexFunc(str[1:], isWhitespace)
    if index == -1 {
        return CommandMessage{Command: str[1:]}
    }

    cmd := str[1 : index+1]
    afterCmd := strings.TrimSpace(str[index+2:])

    var args []string
    if isMultiArgs {
        // Only return the first argument
        args = strings.Fields(afterCmd)
    } else {
        args = []string{afterCmd}
    }
    return CommandMessage{Command: cmd, Args: args}
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

    jsonObj := jsonUtil.AnyToJsonObject(event)

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

func IsEventFromUser(event *linebot.Event) bool {
    return event.Source.Type == linebot.EventSourceTypeUser
}

func IsMessageFromUser(event *linebot.Event) bool {
    if event.Type != linebot.EventTypeMessage {
        // not even message event
        return false
    }

    return IsEventFromUser(event)
}

func IsTextMessage(event *linebot.Event) (bool, error) {
    if event.Type != linebot.EventTypeMessage {
        // not even message event
        return false, nil
    }

    messageType, err := getMessageType(event)
    if err != nil {
        return false, err
    }
    return messageType == linebot.MessageTypeText, nil
}

type message struct {
    Type linebot.MessageType `json:"type"`
}

func (l *Line) buildQuickReplySettingsFlexMessage(user model.User, addUpdateMessage bool) (linebot.FlexContainer, error) {
    var jsonBytes []byte
    hasQuickReplyMessage := user.QuickReplyMessage != nil && strings.TrimSpace(*user.QuickReplyMessage) != ""
    if hasQuickReplyMessage {
        jsonBytes = l.quickReplyJsons.QuickReplySettings
    } else {
        jsonBytes = l.quickReplyJsons.QuickReplySettingsNoQuickReply
    }

    // Convert the original JSON to a map[string]interface{}
    jsonMap, err := jsonUtil.JsonToMap(jsonBytes)
    if err != nil {
        l.log.Fatal("Error unmarshalling QuickReplySettings JSON: ", err)
    }

    if hasQuickReplyMessage {
        // substitute current quick reply message
        if contents, ok := jsonMap["body"].(map[string]interface{})["contents"]; ok {
            if contentsArr, ok := contents.([]interface{}); ok {
                if _, ok := contentsArr[2].(map[string]interface{})["contents"]; ok {
                    jsonMap["body"].(map[string]interface{})["contents"].([]interface{})[2].
                    (map[string]interface{})["contents"].([]interface{})[1].
                    (map[string]interface{})["contents"].([]interface{})[0].
                    (map[string]interface{})["text"] = *user.QuickReplyMessage
                }
            }
        }

        // substitute update button fill with current quick reply message
        if contents, ok := jsonMap["footer"].(map[string]interface{})["contents"]; ok {
            if _, ok := contents.([]interface{}); ok {
                jsonMap["footer"].(map[string]interface{})["contents"].([]interface{})[0].(map[string]interface{})["action"].(map[string]interface{})["fillInText"] = util.UpdateQuickReplyMessageCmd + " " + *user.QuickReplyMessage
            }
        }
    }

    if addUpdateMessage {
        // Convert the original JSON to a map[string]interface{}
        quickReplyUpdatedMessageTextBox, err := jsonUtil.JsonToMap(l.quickReplyJsons.QuickReplyMessageUpdatedTextBox)
        if err != nil {
            l.log.Fatal("Error unmarshalling QuickReplyMessageUpdatedTextBox JSON: ", err)
        }

        // insert quick reply updated message in contents array
        if contents, ok := jsonMap["body"].(map[string]interface{})["contents"]; ok {
            if contentsArr, ok := contents.([]interface{}); ok {
                if subContents, ok := contentsArr[2].(map[string]interface{})["contents"]; ok {
                    if subContentsArr, ok := subContents.([]interface{}); ok {
                        // we know there's only 2 elements prior to insertion
                        contentsArr[2].(map[string]interface{})["contents"] = append(subContentsArr[:1], quickReplyUpdatedMessageTextBox, subContentsArr[1])
                    }
                }
            }
        }
    }

    outputJsonBytes, err := json.Marshal(jsonMap)
    if err != nil {
        l.log.Error("Error marshalling output JSON in buildQuickReplySettingsFlexMessage: ", err)
        return nil, err
    }

    // DEBUG
    l.log.Debug("outputJsonBytes: ", string(outputJsonBytes))

    flexContainer, err := linebot.UnmarshalFlexMessageJSON(outputJsonBytes)
    if err != nil {
        l.log.Error("Error occurred during linebot.UnmarshalFlexMessageJSON in buildQuickReplySettingsFlexMessage: ", err)
        return nil, err
    }

    return flexContainer, nil
}

func (l *Line) buildReviewFlexMessage(review model.Review) (linebot.FlexContainer, error) {
    // Convert the original JSON to a map[string]interface{}
    reviewMsgJson, err := jsonUtil.JsonToMap(l.reviewMessageJsons.ReviewMessage)
    if err != nil {
        l.log.Debug("Error unmarshalling reviewMessage JSON: ", err)
        return nil, err
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
        return nil, err
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
        return nil, err
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
        return nil, err
    }
    reviewMsgFlexContainer, err := linebot.UnmarshalFlexMessageJSON(reviewMsgJsonBytes)
    if err != nil {
        l.log.Error("Error occurred during linebot.UnmarshalFlexMessageJSON: ", err)
        return nil, err
    }

    return reviewMsgFlexContainer, nil
}
