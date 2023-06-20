package model

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "strings"
    "time"
)

type User struct {
    UserId                string                `dynamodbav:"userId"` // partition key
    CreatedAt             time.Time             `dynamodbav:"createdAt,unixtime"`
    LineId                *string               `dynamodbav:"lineId,omitempty"`
    LineUsername          string                `dynamodbav:"lineUsername"`
    LineProfilePictureUrl *string               `dynamodbav:"lineProfilePicture,omitempty" validate:"url"`
    Language              *string               `dynamodbav:"language,omitempty"`
    ZapierReplyWebhook    *string               `dynamodbav:"zapierReplyWebhook,omitempty" validate:"url"` // to be filled by PM during user onboarding
    SubscriptionTier      enum.SubscriptionTier `dynamodbav:"subscriptionTier"`
    ExpireAt              *time.Time            `dynamodbav:"expireAt,omitempty,unixtime"`
    LastUpdated           time.Time             `dynamodbav:"lastUpdated,unixtime"`
    QuickReplyMessage     *string               `dynamodbav:"quickReplyMessage,omitempty"`
}

func NewUser(lineUserId string,
    lineUserProfile linebot.UserProfileResponse,
    createdAt time.Time) User {
    user := User{
        UserId:                lineUserId,
        SubscriptionTier:      enum.SubscriptionTierBeta,
        LineUsername:          lineUserProfile.DisplayName,
        LineProfilePictureUrl: &lineUserProfile.PictureURL,
        Language:              &lineUserProfile.Language,
        CreatedAt:             createdAt,
        LastUpdated:           createdAt,
    }

    return user
}

func (u User) GetKey() map[string]dynamodb.AttributeValue {
    userIdAttributeValue, err := dynamodbattribute.Marshal(u.UserId)
    if err != nil {
        panic(err)
    }

    uniqueId := util.DefaultUniqueId
    return map[string]dynamodb.AttributeValue{
        "userId": {
            S: userIdAttributeValue.S,
        },
        "uniqueId": {
            S: &uniqueId,
        }}
}

func BuildUserDdbKey(userId string) map[string]*dynamodb.AttributeValue {
    uniqueId := util.DefaultUniqueId
    return map[string]*dynamodb.AttributeValue{
        "userId": {
            S: &userId,
        },
        "uniqueId": {
            S: &uniqueId,
        }}
}

func (u User) GetFinalQuickReplyMessage(review Review) string {
    if util.IsEmptyStringPtr(u.QuickReplyMessage) {
        return ""
    }

    return strings.ReplaceAll(*u.QuickReplyMessage, "{評價者}", review.ReviewerName)
}
