package lineUtil

import (
    "encoding/json"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "net/url"
    "strings"
    "unicode"
)

const CannotUseLineEmojiMessage = "暫不支援LINE Emoji，但是您可以考慮使用 Unicode emoji （比如👍🏻）。️很抱歉為您造成不便。"

func IsReviewReplyMessage(message string) bool {
    return strings.HasPrefix(message, "@")
}

// CommandMessage format: "/<command> <args>"
// e.g., "/Help xxx"
type CommandMessage struct {
    Command string
    Args    []string
}

// ParseCommandMessage parses a command message. Unless isMultiArgs is true, all the remaining text after the first command is treated as a single argument.
func ParseCommandMessage(str string, isMultiArgs bool) CommandMessage {
    if !strings.HasPrefix(str, "/") {
        return CommandMessage{}
    }

    // Find the first whitespace character after '/'
    index := strings.IndexFunc(str[1:], isWhitespace)
    if index == -1 {
        return CommandMessage{Command: str[1:], Args: []string{""}} // Return the remaining text after '/' as command
    }

    cmd := str[1 : index+1]
    trimmedCmd := strings.TrimSpace(str[index+2:])

    var args []string
    if isMultiArgs {
        // Only return the first argument
        args = strings.Fields(trimmedCmd)
    } else {
        args = []string{trimmedCmd}
    }
    return CommandMessage{Command: cmd, Args: args}
}

func ParseReplyMessage(str string) (model.Reply, error) {
    if !strings.HasPrefix(str, "@") {
        return model.Reply{}, fmt.Errorf("message is not a reply message: %s", str)
    }

    // Find the first whitespace character after '@'
    index := strings.IndexFunc(str[1:], isWhitespace)
    if index == -1 {
        return model.NewReply(_type.NewReviewId(str[1:]), "") // Return the remaining text after '@' as ReviewId
    }

    reviewID := str[1 : index+1]
    replyMsg := strings.TrimSpace(str[index+2:])

    return model.NewReply(_type.NewReviewId(reviewID), replyMsg)
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

func (l *Line) buildQuickReplySettingsFlexMessage(autoQuickReplyEnabled bool, quickReplyMessage *string) (linebot.FlexContainer, error) {
    jsonMap, err := jsonUtil.JsonToMap(l.quickReplyJsons.QuickReplySettings)
    if err != nil {
        l.log.Debug("Error unmarshalling QuickReplySettings JSON: ", err)
        return nil, err
    }

    // update quick reply message text box
    quickReplyMessageDisplayed := " "
    if !util.IsEmptyStringPtr(quickReplyMessage) {
        quickReplyMessageDisplayed = *quickReplyMessage
    }
    // body -> contents[2] -> contents[1] -> contents[0] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = quickReplyMessageDisplayed
    // body -> contents[2] -> contents[1] -> action -> fillInText
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = util.BuildMessageCmdPrefix(util.UpdateQuickReplyMessageCmd) + quickReplyMessageDisplayed

    // update auto quick reply toggle
    // body -> contents[3] -> contents[0] -> contents[1] -> url
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["url"] = util.GetToggleUrl(autoQuickReplyEnabled)

    // DEBUG
    l.log.Debug("user.AutoQuickReplyEnabled: ", autoQuickReplyEnabled)
    l.log.Debug("end jsonMap in buildQuickReplySettingsFlexMessage: ", jsonUtil.AnyToJson(jsonMap))

    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildReviewFlexMessage(review model.Review, user model.User) (linebot.FlexContainer, error) {
    // Convert the original JSON to a map[string]interface{}
    jsonMap, err := jsonUtil.JsonToMap(l.reviewMessageJsons.ReviewMessage)
    if err != nil {
        l.log.Debug("Error unmarshalling reviewMessage JSON: ", err)
        return nil, err
    }

    // update review message
    isEmptyReview := review.Review == ""
    var reviewMessage string
    if isEmptyReview {
        reviewMessage = "（無文字內容）"
    } else {
        reviewMessage = review.Review
    }

    if contents, ok := jsonMap["body"].(map[string]interface{})["contents"]; ok {
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

    if contents, ok := jsonMap["body"].(map[string]interface{})["contents"]; ok {
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

    // Modify reviewer and timestamp
    if contents, ok := jsonMap["body"].(map[string]interface{})["contents"]; ok {
        if contentsArr, ok := contents.([]interface{}); ok {
            if subContents, ok := contentsArr[2].(map[string]interface{})["contents"]; ok {
                if subContentsArr, ok := subContents.([]interface{}); ok {
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
    ReplyMessagePrefix := fmt.Sprintf("@%s ", review.ReviewId.String())
    fillInText := ReplyMessagePrefix + "感謝…"
    if contents, ok := jsonMap["footer"].(map[string]interface{})["contents"]; ok {
        if contentsArr, ok := contents.([]interface{}); ok {
            if action, ok := contentsArr[1].(map[string]interface{})["action"]; ok {
                action.(map[string]interface{})["fillInText"] = fillInText
            }
        }
    }

    // update AI reply button
    // // remove AI reply button (3rd element in contents array) if review is empty
    if isEmptyReview {
        if contents, ok := jsonMap["footer"].(map[string]interface{})["contents"]; ok {
            if contentsArr, ok := contents.([]interface{}); ok {
                jsonMap["footer"].(map[string]interface{})["contents"] = append(contentsArr[:2])
            }
        }
    } else {
        jsonMap["footer"].(map[string]interface{})["contents"].([]interface{})[2].
        (map[string]interface{})["action"].
        (map[string]interface{})["data"] = "/NewReview/GenerateAiReply/" + review.ReviewId.String()
    }

    // update quick reply button
    // must be done LAST because it will remove the quick reply button if the quick reply message is empty
    quickReplyMsg := user.GetFinalQuickReplyMessage(review)
    if contents, ok := jsonMap["footer"].(map[string]interface{})["contents"]; ok {
        if contentsArr, ok := contents.([]interface{}); ok {
            if util.IsEmptyString(quickReplyMsg) {
                // remove quick reply button
                jsonMap["footer"].(map[string]interface{})["contents"] = append(contentsArr[1:])
            } else {
                // update quick reply message in button
                jsonMap["footer"].(map[string]interface{})["contents"].([]interface{})[0].
                (map[string]interface{})["action"].
                (map[string]interface{})["fillInText"] = ReplyMessagePrefix + quickReplyMsg
            }
        }
    }

    // Convert the map to LINE flex message
    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildAiGeneratedReplyFlexMessage(review model.Review, aiReply string) (linebot.FlexContainer, error) {
    jsonMap, err := jsonUtil.JsonToMap(l.aiReplyJsons.AiReplyResult)
    if err != nil {
        l.log.Debug("Error unmarshalling AiReplyResult JSON: ", err)
        return nil, err
    }

    // update reviewer name
    // body -> contents[2] -> contents[0] -> contents[1] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["text"] = review.ReviewerName

    // update review body
    // body -> contents[2] -> contents[1] -> contents[1] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["text"] = review.Review

    // update ai reply
    // body -> contents[3] -> contents[0] -> contents[0] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = aiReply

    // update buttons
    // footer -> contents[0] -> action -> fillInText
    jsonMap["footer"].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = fmt.Sprintf("@%s %s", review.ReviewId.String(), aiReply)

    // footer -> contents[1] -> action -> data
    jsonMap["footer"].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = "/AiReply/GenerateAiReply/" + review.ReviewId.String()

    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildAiReplySettingsFlexMessage(user model.User, business *model.Business) (linebot.FlexContainer, error) {
    // TODO: [INT-91] Remove backfill logic once all users have been backfilled
    var businessDescriptionDb *string
    var keywordEnabledDb bool
    var keywordsDb *string
    if business == nil {
        businessDescriptionDb = user.BusinessDescription
        if user.KeywordEnabled == nil {
            l.log.Errorf("User %s has no KeywordEnabled field but has not been backfilled", user.UserId)
            keywordEnabledDb = false
        } else {
            keywordEnabledDb = *user.KeywordEnabled
        }
        keywordsDb = user.Keywords
    } else {
        businessDescriptionDb = business.BusinessDescription
        keywordEnabledDb = business.KeywordEnabled
        keywordsDb = business.Keywords
    }

    // Convert the original JSON to a map[string]interface{}
    jsonMap, err := jsonUtil.JsonToMap(l.aiReplyJsons.AiReplySettings)
    if err != nil {
        l.log.Fatal("Error unmarshalling QuickReplySettings JSON: ", err)
    }

    // substitute business description
    var businessDescription string
    if util.IsEmptyStringPtr(businessDescriptionDb) {
        businessDescription = " "
    } else {
        businessDescription = *businessDescriptionDb

        // update fillInText
        // body -> contents[2] -> contents[2] -> action -> fillInText
        jsonMap["body"].
        (map[string]interface{})["contents"].([]interface{})[2].
        (map[string]interface{})["contents"].([]interface{})[2].
        (map[string]interface{})["action"].
        (map[string]interface{})["fillInText"] = util.BuildMessageCmdPrefix(util.UpdateBusinessDescriptionMessageCmd) + businessDescription
    }
    // body -> contents[2] -> contents[2] -> contents[0] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = businessDescription

    // substitute emoji toggle
    // body -> contents[3] -> contents[0] -> contents[1] -> url
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["url"] = util.GetToggleUrl(user.EmojiEnabled)

    // substitute signature toggle
    // body -> contents[4] -> contents[0] -> contents[1] -> url
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["url"] = util.GetToggleUrl(user.SignatureEnabled)

    // substitute signature
    var signature string
    if util.IsEmptyStringPtr(user.Signature) {
        signature = " "
    } else {
        signature = *user.Signature

        // update fillInText
        // body -> contents[4] -> contents[3] -> action -> fillInText
        jsonMap["body"].
        (map[string]interface{})["contents"].([]interface{})[4].
        (map[string]interface{})["contents"].([]interface{})[3].
        (map[string]interface{})["action"].
        (map[string]interface{})["fillInText"] = util.BuildMessageCmdPrefix(util.UpdateSignatureMessageCmd) + signature
    }
    // body -> contents[4] -> contents[3] -> contents[0] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = signature

    // substitute keyword toggle
    // body -> contents[5] -> contents[0] -> contents[1] -> url
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[5].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
        // TODO: [INT-88]
    (map[string]interface{})["url"] = util.GetToggleUrl(keywordEnabledDb)

    // substitute keywords
    var keywords string
    // TODO: [INT-88]
    if util.IsEmptyStringPtr(keywordsDb) {
        keywords = " "
    } else {
        // TODO: [INT-88]
        keywords = *keywordsDb

        // body -> contents[5] -> contents[3] -> action -> fillInText
        jsonMap["body"].
        (map[string]interface{})["contents"].([]interface{})[5].
        (map[string]interface{})["contents"].([]interface{})[3].
        (map[string]interface{})["action"].
        (map[string]interface{})["fillInText"] = util.BuildMessageCmdPrefix(util.UpdateKeywordsMessageCmd) + keywords
    }
    // body -> contents[5] -> contents[3] -> contents[0] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[5].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = keywords

    // substitute service recommendation toggle
    // body -> contents[6] -> contents[0] -> contents[1] -> url
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[6].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["url"] = util.GetToggleUrl(user.ServiceRecommendationEnabled)

    // substitute service recommendation
    var serviceRecommendation string
    if util.IsEmptyStringPtr(user.ServiceRecommendation) {
        serviceRecommendation = " "
    } else {
        serviceRecommendation = *user.ServiceRecommendation

        // body -> contents[6] -> contents[3] -> action -> fillInText
        jsonMap["body"].
        (map[string]interface{})["contents"].([]interface{})[6].
        (map[string]interface{})["contents"].([]interface{})[3].
        (map[string]interface{})["action"].
        (map[string]interface{})["fillInText"] = util.BuildMessageCmdPrefix(util.UpdateRecommendationMessageCmd) + serviceRecommendation
    }
    // body -> contents[5] -> contents[3] -> contents[0] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[6].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = serviceRecommendation

    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildAuthRequestFlexMessage(userId string, authRedirectUrl string) (linebot.FlexContainer, error) {
    jsonMap, err := jsonUtil.JsonToMap(l.authJsons.AuthRequest)
    if err != nil {
        l.log.Debug("Error unmarshalling AuthRequest JSON: ", err)
        return nil, err
    }

    // substitute auth redirect url
    // footer -> contents[0] -> action -> uri
    oldUri := jsonMap["footer"].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["action"].
    (map[string]interface{})["uri"].(string)

    // replace the redirect_uri query parameter in the uri with authRedirectUrl
    uri, err := finalizeAuthUri(oldUri, userId, authRedirectUrl)
    if err != nil {
        return nil, err
    }

    l.log.Debug("AuthRequest URI: ", uri)

    jsonMap["footer"].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["action"].
    (map[string]interface{})["uri"] = uri

    return l.jsonMapToLineFlexContainer(jsonMap)
}

func finalizeAuthUri(uri string, userId string, authRedirectUrl string) (string, error) {
    parsedURL, err := url.Parse(uri)
    if err != nil {
        return "", err
    }

    queryParams, err := url.ParseQuery(parsedURL.RawQuery)
    if err != nil {
        return "", err
    }

    queryParams.Set("redirect_uri", authRedirectUrl)
    queryParams.Set("state", userId)
    parsedURL.RawQuery = queryParams.Encode()

    return parsedURL.String(), nil
}

func (l *Line) jsonMapToLineFlexContainer(jsonMap map[string]interface{}) (linebot.FlexContainer, error) {
    // Convert the map to LINE flex message
    // first convert back to json
    jsonBytes, err := json.Marshal(jsonMap)
    if err != nil {
        l.log.Error("Error marshalling JSON: ", err)
        return nil, err
    }
    flexContainer, err := linebot.UnmarshalFlexMessageJSON(jsonBytes)
    if err != nil {
        l.log.Error("Error occurred during linebot.UnmarshalFlexMessageJSON: ", err)
        return nil, err
    }

    return flexContainer, nil
}
