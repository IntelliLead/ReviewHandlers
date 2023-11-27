package model

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "strings"
    "time"
)

type Business struct {
    BusinessId            bid.BusinessId `dynamodbav:"businessId"` // partition key
    BusinessName          string         `dynamodbav:"businessName"`
    UserIds               []string       `dynamodbav:"userIds,stringset,omitemptyelem"`
    BusinessDescription   *string        `dynamodbav:"businessDescription,omitempty"`
    Keywords              *string        `dynamodbav:"keywords,omitempty"`
    KeywordEnabled        bool           `dynamodbav:"keywordEnabled"` // FAC for keywords
    QuickReplyMessage     *string        `dynamodbav:"quickReplyMessage,omitempty"`
    AutoQuickReplyEnabled bool           `dynamodbav:"autoQuickReplyEnabled"` // FAC for auto quick reply
    CreatedAt             time.Time      `dynamodbav:"createdAt,unixtime"`
    LastUpdated           time.Time      `dynamodbav:"lastUpdated,unixtime"`
    LastUpdatedBy         string         `dynamodbav:"lastUpdatedBy"`
}

func NewBusiness(businessId bid.BusinessId,
    businessName string,
    userId string) Business {

    return Business{
        BusinessId:   businessId,
        BusinessName: businessName,
        UserIds: []string{
            userId,
        },
        KeywordEnabled:        false,
        AutoQuickReplyEnabled: false,
        CreatedAt:             time.Now(),
        LastUpdated:           time.Now(),
        LastUpdatedBy:         userId,
    }
}

func BuildDdbBusinessKey(businessId bid.BusinessId) map[string]types.AttributeValue {
    uniqueId := util.DefaultUniqueId
    return map[string]types.AttributeValue{
        "businessId": &types.AttributeValueMemberS{Value: businessId.String()},
        "uniqueId":   &types.AttributeValueMemberS{Value: uniqueId},
    }
}

// GetFinalQuickReplyMessage returns the final quick reply message to be sent
// It replaces the {評論人} placeholder with the reviewer's name.
func (b Business) GetFinalQuickReplyMessage(review Review) string {
    if util.IsEmptyStringPtr(b.QuickReplyMessage) {
        return ""
    }

    return strings.ReplaceAll(*b.QuickReplyMessage, "{評論人}", review.ReviewerName)
}
