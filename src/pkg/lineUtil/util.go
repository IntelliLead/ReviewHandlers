package lineUtil

import (
    "encoding/json"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "net/url"
)

const CannotUseLineEmojiMessage = "暫不支援LINE Emoji，但是您可以考慮使用 Unicode emoji （比如👍🏻）。️很抱歉為您造成不便。"

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

    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildReviewFlexMessage(review model.Review, quickReplyMessage string) (linebot.FlexContainer, error) {
    // Convert the original JSON to a map[string]interface{}
    jsonMap, err := jsonUtil.JsonToMap(l.reviewMessageJsons.ReviewMessage)
    if err != nil {
        l.log.Debug("Error unmarshalling reviewMessage JSON: ", err)
        return nil, err
    }

    // update review message
    var reviewMessage string
    isEmptyReview := util.IsEmptyStringPtr(review.Review)
    if isEmptyReview {
        reviewMessage = "（無文字內容）"
    } else {
        reviewMessage = *review.Review
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
    replyMessagePrefix := fmt.Sprintf("@%s ", review.ReviewId.String())
    if contents, ok := jsonMap["footer"].(map[string]interface{})["contents"]; ok {
        if contentsArr, ok := contents.([]interface{}); ok {
            if action, ok := contentsArr[1].(map[string]interface{})["action"]; ok {
                action.(map[string]interface{})["fillInText"] = replyMessagePrefix
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
    if contents, ok := jsonMap["footer"].(map[string]interface{})["contents"]; ok {
        if contentsArr, ok := contents.([]interface{}); ok {
            if util.IsEmptyString(quickReplyMessage) {
                // remove quick reply button
                jsonMap["footer"].(map[string]interface{})["contents"] = append(contentsArr[1:])
            } else {
                // update quick reply message in button
                jsonMap["footer"].(map[string]interface{})["contents"].([]interface{})[0].
                (map[string]interface{})["action"].
                (map[string]interface{})["fillInText"] = replyMessagePrefix + quickReplyMessage
            }
        }
    }

    // Convert the map to LINE flex message
    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildAiGeneratedReplyFlexMessage(review model.Review, aiReply string, generateAuthorName string) (linebot.FlexContainer, error) {
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

    // update generate author name
    // body -> contents[4] -> contents[1] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["text"] = generateAuthorName

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

func (l *Line) buildAiReplySettingsFlexMessage(user model.User, business model.Business) (linebot.FlexContainer, error) {
    // Convert the original JSON to a map[string]interface{}
    jsonMap, err := jsonUtil.JsonToMap(l.aiReplyJsons.AiReplySettings)
    if err != nil {
        l.log.Fatal("Error unmarshalling QuickReplySettings JSON: ", err)
    }

    // substitute business description
    var businessDescription string
    if util.IsEmptyStringPtr(business.BusinessDescription) {
        businessDescription = " "
    } else {
        businessDescription = *business.BusinessDescription

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
    (map[string]interface{})["url"] = util.GetToggleUrl(business.KeywordEnabled)

    // substitute keywords
    var keywords string
    if util.IsEmptyStringPtr(business.Keywords) {
        keywords = " "
    } else {
        keywords = *business.Keywords

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

func (l *Line) buildReviewRepliedNotificationMessage(review model.Review, reply string, replierName string, isAutoReply bool, businessName *string) (linebot.FlexContainer, error) {
    jsonMap, err := jsonUtil.JsonToMap(l.notificationJsons.ReviewReplied)
    if err != nil {
        l.log.Debug("Error unmarshalling ReviewReplied JSON: ", err)
        return nil, err
    }

    // substitute title to whether it is auto-reply
    if isAutoReply {
        // body -> contents[0] -> contents[0] -> text
        jsonMap["body"].
        (map[string]interface{})["contents"].([]interface{})[0].
        (map[string]interface{})["contents"].([]interface{})[0].
        (map[string]interface{})["text"] = "評論自動回覆通知"
    }

    // substitute business name if available, otherwise omit the business name section
    if !util.IsEmptyStringPtr(businessName) {
        // body -> contents[0] -> contents[1] -> text
        jsonMap["body"].
        (map[string]interface{})["contents"].([]interface{})[0].
        (map[string]interface{})["contents"].([]interface{})[1].
        (map[string]interface{})["text"] = *businessName
    } else {
        jsonMap["body"].
        (map[string]interface{})["contents"].([]interface{})[0].
        (map[string]interface{})["contents"] = jsonMap["body"].
        (map[string]interface{})["contents"].([]interface{})[0].
        (map[string]interface{})["contents"].([]interface{})[:1]
    }

    // substitute review
    reviewMessage := "（無文字內容）"
    if !util.IsEmptyStringPtr(review.Review) {
        reviewMessage = *review.Review
    }
    // body -> contents[1] -> contents[0] -> contents[1] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["text"] = reviewMessage

    // substitute reviewer name
    // body -> contents[1] -> contents[1] -> contents[1] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["text"] = review.ReviewerName

    // substitute reply
    // body -> contents[1] -> contents[3] -> contents[1] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["text"] = reply

    // substitute replier name
    // body -> contents[1] -> contents[4] -> contents[1] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["text"] = replierName

    // substitute button fillInText
    // footer -> contents[0] -> action -> fillInText
    jsonMap["footer"].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = fmt.Sprintf("@%s %s", review.ReviewId.String(), reply)

    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildQuickReplySettingsUpdatedNotificationMessage(updaterName string) (linebot.FlexContainer, error) {
    jsonMap, err := jsonUtil.JsonToMap(l.notificationJsons.QuickReplySettingsUpdated)
    if err != nil {
        l.log.Debug("Error unmarshalling QuickReplySettingsUpdated JSON: ", err)
        return nil, err
    }

    // substitute updater name
    // body -> contents[1] -> contents[0] -> contents[1] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["text"] = updaterName

    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildAiReplySettingsUpdatedNotificationMessage(updaterName string) (linebot.FlexContainer, error) {
    jsonMap, err := jsonUtil.JsonToMap(l.notificationJsons.AiReplySettingsUpdated)
    if err != nil {
        l.log.Debug("Error unmarshalling AiReplySettingsUpdated JSON: ", err)
        return nil, err
    }

    // substitute updater name
    // body -> contents[1] -> contents[0] -> contents[1] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["text"] = updaterName

    return l.jsonMapToLineFlexContainer(jsonMap)
}
