package model

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "time"
)

type Business struct {
    BusinessId          string    `dynamodbav:"businessId"` // partition key
    BusinessName        string    `dynamodbav:"businessName"`
    UserIds             []string  `dynamodbav:"userIds,stringset,omitemptyelem"`
    BusinessDescription *string   `dynamodbav:"businessDescription,omitempty"`
    Keywords            *string   `dynamodbav:"keywords,omitempty"`
    KeywordEnabled      bool      `dynamodbav:"keywordEnabled"` // FAC for keywords
    CreatedAt           time.Time `dynamodbav:"createdAt,unixtime"`
    LastUpdated         time.Time `dynamodbav:"lastUpdated,unixtime"`
    LastUpdatedBy       string    `dynamodbav:"lastUpdatedBy"`
    Google              *Google   `dynamodbav:"google,omitemptyelem"`
}

func NewBusiness(businessId string,
    businessName string,
    google Google,
    userId string) Business {

    return Business{
        BusinessId:   businessId,
        BusinessName: businessName,
        UserIds: []string{
            userId,
        },
        KeywordEnabled: false,
        CreatedAt:      time.Now(),
        LastUpdated:    time.Now(),
        LastUpdatedBy:  userId,
        Google:         &google,
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
