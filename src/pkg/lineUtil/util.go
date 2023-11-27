package lineUtil

import (
    "encoding/json"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "net/url"
)

const CannotUseLineEmojiMessage = "暫不支援LINE Emoji，但是您可以考慮使用 Unicode emoji （比如👍🏻）。"

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

// buildQuickReplySettingsFlexMessageForMultiBusiness builds a LINE flex message for quick reply settings for multi-business
// orderedBusinesses must be sorted by businessIdIndex (i.e., the order as appear in sorted user.BusinessIds)
// activeBusinessId must be in orderedBusinesses
func (l *Line) buildQuickReplySettingsFlexMessageForMultiBusiness(
    orderedBusinesses []model.Business,
    activeBusinessId bid.BusinessId,
) (linebot.FlexContainer, error) {
    jsonMap, err := jsonUtil.JsonToMap(l.quickReplyJsons.QuickReplySettingsMultiBusiness)
    if err != nil {
        l.log.Error("Error unmarshalling QuickReplySettingsMultiBusiness JSON: ", err)
        return nil, err
    }

    // find index of active business
    activeBusinessIndex := -1
    for i, business := range orderedBusinesses {
        if business.BusinessId == activeBusinessId {
            activeBusinessIndex = i
            break
        }
    }
    if activeBusinessIndex == -1 {
        l.log.Error("Error finding active business index. activeBusinessId is not in orderedBusinesses: ", activeBusinessId)
        return nil, fmt.Errorf("activeBusinessId is not in orderedBusinesses: %s", activeBusinessId)
    }

    business := orderedBusinesses[activeBusinessIndex]

    // update business name for first bubble
    // contents[0] -> hero -> contents[0] -> text
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["hero"].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = business.BusinessName

    // update quick reply message text box
    quickReplyMessageDisplayed := " "
    if !util.IsEmptyStringPtr(business.QuickReplyMessage) {
        quickReplyMessageDisplayed = *business.QuickReplyMessage
    }
    // contents[0] -> body -> contents[2] -> contents[1] -> contents[0] -> text
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = quickReplyMessageDisplayed

    // update quick reply message button
    quickReplyPostbackData := fmt.Sprintf("/QuickReply/%s/EditQuickReplyMessage", business.BusinessId)
    quickReplyFillInText := fmt.Sprintf("/%s/%d %s", util.UpdateQuickReplyMessageCmd, activeBusinessIndex, quickReplyMessageDisplayed)
    // contents[0] -> body -> contents[2] -> contents[1] -> action -> data
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = quickReplyPostbackData
    // contents[0] -> body -> contents[2] -> contents[1] -> action -> fillInText
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = quickReplyFillInText

    // update auto quick reply toggle
    autoQuickReplyTogglePostbackData := fmt.Sprintf("/QuickReply/%s/Toggle/AutoReply", business.BusinessId)
    // contents[0] -> body -> contents[3] -> contents[0] -> contents[1] -> url
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["url"] = util.GetToggleUrl(business.AutoQuickReplyEnabled)
    // contents[0] -> body -> contents[3] -> contents[0] -> contents[1] -> action -> data
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = autoQuickReplyTogglePostbackData

    // update other business bubbles
    otherBusinessBubbleTemplate, err := util.DeepCopy(jsonMap["contents"].([]interface{})[1])
    if err != nil {
        l.log.Error("Error copying otherBusinessBubbleTemplate: ", err)
        return nil, err
    }
    // remove template as 2nd bubble
    jsonMap["contents"] = jsonMap["contents"].([]interface{})[:1]
    for businessIndex, business := range orderedBusinesses {
        if businessIndex == activeBusinessIndex {
            continue
        }

        otherBusinessJsonMap, _ := util.DeepCopy(otherBusinessBubbleTemplate)

        // update business name
        // otherBusinessJsonMap -> body -> contents[0] -> text
        otherBusinessJsonMap.(map[string]interface{})["body"].
        (map[string]interface{})["contents"].([]interface{})[0].
        (map[string]interface{})["text"] = business.BusinessName

        // update switch business button
        // otherBusinessJsonMap -> body -> contents[1] -> contents[0] -> action -> data
        otherBusinessJsonMap.(map[string]interface{})["body"].
        (map[string]interface{})["contents"].([]interface{})[1].
        (map[string]interface{})["contents"].([]interface{})[0].
        (map[string]interface{})["action"].
        (map[string]interface{})["data"] = fmt.Sprintf("/QuickReply/%s/UpdateActiveBusiness", business.BusinessId)

        jsonMap["contents"] = append(jsonMap["contents"].([]interface{}), otherBusinessJsonMap)
    }

    return l.jsonMapToLineFlexContainer(jsonMap)
}

// buildQuickReplySettingsFlexMessage builds a LINE flex message for quick reply settings
func (l *Line) buildQuickReplySettingsFlexMessage(business model.Business) (linebot.FlexContainer, error) {
    jsonMap, err := jsonUtil.JsonToMap(l.quickReplyJsons.QuickReplySettings)
    if err != nil {
        l.log.Debug("Error unmarshalling QuickReplySettings JSON: ", err)
        return nil, err
    }

    // true for single business
    businessIdIndex := 0

    // update quick reply message text box
    quickReplyMessageDisplayed := " "
    if !util.IsEmptyStringPtr(business.QuickReplyMessage) {
        quickReplyMessageDisplayed = *business.QuickReplyMessage
    }
    // body -> contents[2] -> contents[1] -> contents[0] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = quickReplyMessageDisplayed

    // update quick reply message button
    quickReplyPostbackData := fmt.Sprintf("/QuickReply/%s/EditQuickReplyMessage", business.BusinessId)
    quickReplyFillInText := fmt.Sprintf("/%s/%d %s", util.UpdateQuickReplyMessageCmd, businessIdIndex, quickReplyMessageDisplayed)
    // body -> contents[2] -> contents[1] -> action -> data
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = quickReplyPostbackData
    // body -> contents[2] -> contents[1] -> action -> fillInText
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = quickReplyFillInText

    // update auto quick reply toggle
    autoQuickReplyTogglePostbackData := fmt.Sprintf("/QuickReply/%s/Toggle/AutoReply", business.BusinessId)
    // body -> contents[3] -> contents[0] -> contents[1] -> url
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["url"] = util.GetToggleUrl(business.AutoQuickReplyEnabled)
    // body -> contents[3] -> contents[0] -> contents[1] -> action -> data
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = autoQuickReplyTogglePostbackData

    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildReviewFlexMessage(review model.Review, quickReplyMessage string, businessId bid.BusinessId, businessIdIndex int, businessName *string) (linebot.FlexContainer, error) {
    // Convert the original JSON to a map[string]interface{}
    jsonMap, err := jsonUtil.JsonToMap(l.reviewMessageJsons.ReviewMessage)
    if err != nil {
        l.log.Debug("Error unmarshalling reviewMessage JSON: ", err)
        return nil, err
    }

    // update business name if exist
    if util.IsEmptyStringPtr(businessName) {
        delete(jsonMap, "hero")
    } else {
        jsonMap["hero"].(map[string]interface{})["contents"].([]interface{})[0].(map[string]interface{})["text"] = *businessName
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
    urid, err := model.NewUserReviewId(&businessIdIndex, review.ReviewId)
    if err != nil {
        l.log.Error("Error creating UserReviewId: ", err)
        return nil, err
    }
    replyMessagePrefix := fmt.Sprintf("@%s ", urid.String())
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
        (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/GenerateAiReply/%s/%s", businessId, review.ReviewId.String())
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

func (l *Line) buildReviewFlexMessageForUnauthedUser(review model.Review) (linebot.FlexContainer, error) {
    // Convert the original JSON to a map[string]interface{}
    jsonMap, err := jsonUtil.JsonToMap(l.reviewMessageJsons.ReviewMessage)
    if err != nil {
        l.log.Debug("Error unmarshalling reviewMessage JSON: ", err)
        return nil, err
    }

    // remove hero section because we do not have business name
    delete(jsonMap, "hero")

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

    // Convert the map to LINE flex message
    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildAiGeneratedReplyFlexMessage(review model.Review, aiReply string, generateAuthorName string, businessId bid.BusinessId, businessIdIndex int) (linebot.FlexContainer, error) {
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

    // update 送出回覆 button
    // footer -> contents[0] -> action -> fillInText
    jsonMap["footer"].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = fmt.Sprintf("@%d|%s %s", businessIdIndex, review.ReviewId.String(), aiReply)
    // footer -> contents[0] -> action -> data
    jsonMap["footer"].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/EditReply", businessId)

    // footer -> contents[1] -> action -> data
    jsonMap["footer"].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/GenerateAiReply/%s/%s", businessId, review.ReviewId.String())

    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildAiReplySettingsFlexMessageForMultiBusiness(user model.User, orderedBusinesses []model.Business, activeBusinessId bid.BusinessId) (linebot.FlexContainer, error) {
    // Convert the original JSON to a map[string]interface{}
    jsonMap, err := jsonUtil.JsonToMap(l.aiReplyJsons.AiReplySettingsMultiBusiness)
    if err != nil {
        l.log.Fatal("Error unmarshalling AiReplySettingsMultiBusiness JSON: ", err)
    }

    // find index of active business
    activeBusinessIndex := -1
    for i, business := range orderedBusinesses {
        if business.BusinessId == activeBusinessId {
            activeBusinessIndex = i
            break
        }
    }
    if activeBusinessIndex == -1 {
        l.log.Error("Error finding active business index. activeBusinessId is not in orderedBusinesses: ", activeBusinessId)
        return nil, fmt.Errorf("activeBusinessId is not in orderedBusinesses: %s", activeBusinessId)
    }

    business := orderedBusinesses[activeBusinessIndex]

    // update business name for first bubble
    // contents[0] -> hero -> contents[0] -> text
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["hero"].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = business.BusinessName

    // substitute business description
    var businessDescription string
    if util.IsEmptyStringPtr(business.BusinessDescription) {
        businessDescription = " "
    } else {
        businessDescription = *business.BusinessDescription
    }
    // contents[0] -> body -> contents[2] -> contents[2] -> contents[0] -> text
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = businessDescription
    // update fillInText
    // contents[0] -> body -> contents[2] -> contents[2] -> action -> fillInText
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = fmt.Sprintf("/%s/%d %s", util.UpdateBusinessDescriptionMessageCmd, activeBusinessIndex, businessDescription)
    // contents[0] -> body -> contents[2] -> contents[2] -> action -> data
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/EditBusinessDescription", business.BusinessId)

    // substitute emoji toggle
    // contents[0] -> body -> contents[3] -> contents[0] -> contents[1] -> url
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["url"] = util.GetToggleUrl(user.EmojiEnabled)
    // contents[0] -> body -> contents[3] -> contents[0] -> contents[1] -> action -> data
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/Toggle/Emoji", business.BusinessId)

    // substitute signature toggle
    // contents[0] -> body -> contents[4] -> contents[0] -> contents[1] -> url
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["url"] = util.GetToggleUrl(user.SignatureEnabled)
    // contents[0] -> body -> contents[4] -> contents[0] -> contents[1] -> action -> data
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/Toggle/Signature", business.BusinessId)

    // substitute signature
    var signature string
    if util.IsEmptyStringPtr(user.Signature) {
        signature = " "
    } else {
        signature = *user.Signature
    }
    // update fillInText
    // contents[0] -> body -> contents[4] -> contents[3] -> action -> fillInText
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = fmt.Sprintf("/%s/%d %s", util.UpdateSignatureMessageCmd, activeBusinessIndex, signature)
    // contents[0] -> body -> contents[4] -> contents[3] -> action -> data
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/EditSignature", business.BusinessId)
    // contents[0] -> body -> contents[4] -> contents[3] -> contents[0] -> text
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = signature

    // substitute keyword toggle
    // contents[0] -> body -> contents[5] -> contents[0] -> contents[1] -> url
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[5].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["url"] = util.GetToggleUrl(business.KeywordEnabled)
    // contents[0] -> body -> contents[5] -> contents[0] -> contents[1] -> action -> data
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[5].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/Toggle/Keyword", business.BusinessId)

    // substitute keywords
    var keywords string
    if util.IsEmptyStringPtr(business.Keywords) {
        keywords = " "
    } else {
        keywords = *business.Keywords
    }
    // contents[0] -> body -> contents[5] -> contents[3] -> action -> fillInText
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[5].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = fmt.Sprintf("/%s/%d %s", util.UpdateKeywordsMessageCmd, activeBusinessIndex, keywords)
    // contents[0] -> body -> contents[5] -> contents[3] -> action -> data
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[5].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/EditKeywords", business.BusinessId)
    // contents[0] -> body -> contents[5] -> contents[3] -> contents[0] -> text
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[5].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = keywords

    // substitute service recommendation toggle
    // contents[0] -> body -> contents[6] -> contents[0] -> contents[1] -> url
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[6].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["url"] = util.GetToggleUrl(user.ServiceRecommendationEnabled)
    // contents[0] -> body -> contents[6] -> contents[0] -> contents[1] -> action -> data
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[6].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/Toggle/ServiceRecommendation", business.BusinessId)

    // substitute service recommendation
    var serviceRecommendation string
    if util.IsEmptyStringPtr(user.ServiceRecommendation) {
        serviceRecommendation = " "
    } else {
        serviceRecommendation = *user.ServiceRecommendation
    }
    // contents[0] -> body -> contents[6] -> contents[3] -> action -> fillInText
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[6].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = fmt.Sprintf("/%s/%d %s", util.UpdateRecommendationMessageCmd, activeBusinessIndex, serviceRecommendation)
    // contents[0] -> body -> contents[6] -> contents[3] -> action -> data
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[6].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/EditServiceRecommendations", business.BusinessId)
    // contents[0] -> body -> contents[5] -> contents[3] -> contents[0] -> text
    jsonMap["contents"].([]interface{})[0].
    (map[string]interface{})["body"].
    (map[string]interface{})["contents"].([]interface{})[6].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = serviceRecommendation

    // update other business bubbles
    otherBusinessBubbleTemplate, err := util.DeepCopy(jsonMap["contents"].([]interface{})[1])
    if err != nil {
        l.log.Error("Error copying otherBusinessBubbleTemplate: ", err)
        return nil, err
    }
    // remove template as 2nd bubble
    jsonMap["contents"] = jsonMap["contents"].([]interface{})[:1]
    for businessIndex, business := range orderedBusinesses {
        if businessIndex == activeBusinessIndex {
            continue
        }

        otherBusinessJsonMap, _ := util.DeepCopy(otherBusinessBubbleTemplate)

        // update business name
        // otherBusinessJsonMap -> body -> contents[0] -> text
        otherBusinessJsonMap.(map[string]interface{})["body"].
        (map[string]interface{})["contents"].([]interface{})[0].
        (map[string]interface{})["text"] = business.BusinessName

        // update switch business button
        // otherBusinessJsonMap -> body -> contents[1] -> contents[0] -> action -> data
        otherBusinessJsonMap.(map[string]interface{})["body"].
        (map[string]interface{})["contents"].([]interface{})[1].
        (map[string]interface{})["contents"].([]interface{})[0].
        (map[string]interface{})["action"].
        (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/UpdateActiveBusiness", business.BusinessId)

        jsonMap["contents"] = append(jsonMap["contents"].([]interface{}), otherBusinessJsonMap)
    }

    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildAiReplySettingsFlexMessageForSingleBusiness(user model.User, business model.Business) (linebot.FlexContainer, error) {
    // Convert the original JSON to a map[string]interface{}
    jsonMap, err := jsonUtil.JsonToMap(l.aiReplyJsons.AiReplySettings)
    if err != nil {
        l.log.Fatal("Error unmarshalling QuickReplySettings JSON: ", err)
    }

    // true for single business
    activeBusinessIndex := 0

    // substitute business description
    var businessDescription string
    if util.IsEmptyStringPtr(business.BusinessDescription) {
        businessDescription = " "
    } else {
        businessDescription = *business.BusinessDescription
    }
    // update fillInText
    // body -> contents[2] -> contents[2] -> action -> fillInText
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = fmt.Sprintf("/%s/%d %s", util.UpdateBusinessDescriptionMessageCmd, activeBusinessIndex, businessDescription)
    // body -> contents[2] -> contents[2] -> contents[0] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = businessDescription
    // body -> contents[2] -> contents[2] -> action -> data
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["contents"].([]interface{})[2].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/EditBusinessDescription", business.BusinessId)

    // substitute emoji toggle
    // body -> contents[3] -> contents[0] -> contents[1] -> url
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["url"] = util.GetToggleUrl(user.EmojiEnabled)
    // body -> contents[3] -> contents[0] -> contents[1] -> action -> data
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/Toggle/Emoji", business.BusinessId)

    // substitute signature toggle
    // body -> contents[4] -> contents[0] -> contents[1] -> url
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["url"] = util.GetToggleUrl(user.SignatureEnabled)
    // body -> contents[4] -> contents[0] -> contents[1] -> action -> data
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/Toggle/Signature", business.BusinessId)

    // substitute signature
    var signature string
    if util.IsEmptyStringPtr(user.Signature) {
        signature = " "
    } else {
        signature = *user.Signature
    }
    // update fillInText
    // body -> contents[4] -> contents[3] -> action -> fillInText
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = fmt.Sprintf("/%s/%d %s", util.UpdateSignatureMessageCmd, activeBusinessIndex, signature)
    // body -> contents[4] -> contents[3] -> action -> data
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[4].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/EditSignature", business.BusinessId)
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
    // body -> contents[5] -> contents[0] -> contents[1] -> action -> data
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[5].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/Toggle/Keyword", business.BusinessId)

    // substitute keywords
    var keywords string
    if util.IsEmptyStringPtr(business.Keywords) {
        keywords = " "
    } else {
        keywords = *business.Keywords
    }
    // body -> contents[5] -> contents[3] -> action -> fillInText
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[5].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = fmt.Sprintf("/%s/%d %s", util.UpdateKeywordsMessageCmd, activeBusinessIndex, keywords)
    // body -> contents[5] -> contents[3] -> action -> data
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[5].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/EditKeywords", business.BusinessId)
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
    // body -> contents[6] -> contents[0] -> contents[1] -> action -> data
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[6].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/Toggle/ServiceRecommendation", business.BusinessId)

    // substitute service recommendation
    var serviceRecommendation string
    if util.IsEmptyStringPtr(user.ServiceRecommendation) {
        serviceRecommendation = " "
    } else {
        serviceRecommendation = *user.ServiceRecommendation
    }
    // body -> contents[6] -> contents[3] -> action -> fillInText
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[6].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["action"].
    (map[string]interface{})["fillInText"] = fmt.Sprintf("/%s/%d %s", util.UpdateRecommendationMessageCmd, activeBusinessIndex, serviceRecommendation)
    // body -> contents[6] -> contents[3] -> action -> data
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[6].
    (map[string]interface{})["contents"].([]interface{})[3].
    (map[string]interface{})["action"].
    (map[string]interface{})["data"] = fmt.Sprintf("/AiReply/%s/EditServiceRecommendations", business.BusinessId)
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

func (l *Line) buildReviewRepliedNotificationMessage(review model.Review, reply string, replierName string, isAutoReply bool, businessName string, businessIdIndex int) (linebot.FlexContainer, error) {
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

    // substitute business name
    // hero -> contents[0] -> text
    jsonMap["hero"].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = businessName

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
    (map[string]interface{})["fillInText"] = fmt.Sprintf("@%d|%s %s", businessIdIndex, review.ReviewId.String(), reply)

    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildQuickReplySettingsUpdatedNotificationMessage(updaterName string, businessName string) (linebot.FlexContainer, error) {
    jsonMap, err := jsonUtil.JsonToMap(l.notificationJsons.QuickReplySettingsUpdated)
    if err != nil {
        l.log.Debug("Error unmarshalling QuickReplySettingsUpdated JSON: ", err)
        return nil, err
    }

    // substitute business name
    jsonMap["hero"].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = businessName

    // substitute updater name
    // body -> contents[1] -> contents[0] -> contents[1] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["text"] = updaterName

    return l.jsonMapToLineFlexContainer(jsonMap)
}

func (l *Line) buildAiReplySettingsUpdatedNotificationMessage(updaterName string, businessName string) (linebot.FlexContainer, error) {
    jsonMap, err := jsonUtil.JsonToMap(l.notificationJsons.AiReplySettingsUpdated)
    if err != nil {
        l.log.Debug("Error unmarshalling AiReplySettingsUpdated JSON: ", err)
        return nil, err
    }

    // substitute business name
    jsonMap["hero"].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["text"] = businessName

    // substitute updater name
    // body -> contents[1] -> contents[0] -> contents[1] -> text
    jsonMap["body"].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["contents"].([]interface{})[0].
    (map[string]interface{})["contents"].([]interface{})[1].
    (map[string]interface{})["text"] = updaterName

    return l.jsonMapToLineFlexContainer(jsonMap)
}
