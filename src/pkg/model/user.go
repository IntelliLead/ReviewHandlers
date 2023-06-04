package model

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
    "time"
)

type User struct {
    UserID             string                `dynamodbav:"userId"` // partition key
    CreatedAt          time.Time             `dynamodbav:"createdAt"`
    LineID             *string               `dynamodbav:"lineId,omitempty"`
    ZapierReplyWebhook *string               `dynamodbav:"zapierReplyWebhook,omitempty" validate:"url"` // to be filled by PM during user onboarding
    SubscriptionTier   enum.SubscriptionTier `dynamodbav:"subscriptionTier"`
    ExpireAt           *time.Time            `dynamodbav:"expireAt,omitempty"`
    LastUpdated        time.Time             `dynamodbav:"lastUpdated"`
}

func NewUser(lineUserId string, createdAt time.Time) User {
    user := User{
        UserID:           lineUserId,
        SubscriptionTier: enum.SubscriptionTierBeta,
        CreatedAt:        createdAt,
        LastUpdated:      createdAt,
    }

    return user
}

func (u User) GetKey() map[string]dynamodb.AttributeValue {
    userId, err := dynamodbattribute.Marshal(u.UserID)
    if err != nil {
        panic(err)
    }
    createdAt, err := dynamodbattribute.Marshal(u.CreatedAt)
    if err != nil {
        panic(err)
    }
    return map[string]dynamodb.AttributeValue{
        "userId": {
            S: userId.S,
        },
        "createdAt": {
            N: createdAt.N,
        }}
}
