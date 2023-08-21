package model

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "time"
)

type Business struct {
    BusinessId          string                `dynamodbav:"userId"` // partition key
    UserIds             []string              `dynamodbav:"userIds,omitemptyelem"`
    BusinessDescription *string               `dynamodbav:"businessDescription,omitempty"`
    Keywords            *string               `dynamodbav:"keywords,omitempty"`
    KeywordEnabled      bool                  `dynamodbav:"keywordEnabled"` // FAC for keywords
    SubscriptionTier    enum.SubscriptionTier `dynamodbav:"subscriptionTier"`
    CreatedAt           time.Time             `dynamodbav:"createdAt,unixtime"`
    LastUpdated         time.Time             `dynamodbav:"lastUpdated,unixtime"`
    LastUpdatedBy       string                `dynamodbav:"lastUpdatedBy"`
    Google              *Google               `dynamodbav:"google,omitemptyelem"`
}

func NewBusiness(businessId string) Business {
    return Business{
        CreatedAt:      time.Now(),
        LastUpdated:    time.Now(),
        KeywordEnabled: false,
    }
}

func BuildDdbBusinessKey(userId string) map[string]*dynamodb.AttributeValue {
    uniqueId := util.DefaultUniqueId
    return map[string]*dynamodb.AttributeValue{
        "businessId": {
            S: &userId,
        },
        "uniqueId": {
            S: &uniqueId,
        }}
}
