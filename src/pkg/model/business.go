package model

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "time"
)

// TODO: WIP
type Business struct {
    BusinessId                   string                `dynamodbav:"userId"` // partition key
    UserIds                      []string              `dynamodbav:"userIds"`
    LineId                       *string               `dynamodbav:"lineId,omitempty"`
    LineUsername                 string                `dynamodbav:"lineUsername"`
    LineProfilePictureUrl        *string               `dynamodbav:"lineProfilePicture,omitempty" validate:"url"`
    Language                     *string               `dynamodbav:"language,omitempty"`
    ZapierReplyWebhook           *string               `dynamodbav:"zapierReplyWebhook,omitempty" validate:"url"` // to be filled by PM during user onboarding
    SubscriptionTier             enum.SubscriptionTier `dynamodbav:"subscriptionTier"`
    ExpireAt                     *time.Time            `dynamodbav:"expireAt,omitempty,unixtime"`
    LastUpdated                  time.Time             `dynamodbav:"lastUpdated,unixtime"`
    QuickReplyMessage            *string               `dynamodbav:"quickReplyMessage,omitempty"`
    BusinessDescription          *string               `dynamodbav:"businessDescription,omitempty"`
    EmojiEnabled                 bool                  `dynamodbav:"emojiEnabled"` // FAC for emoji
    Signature                    *string               `dynamodbav:"signature,omitempty"`
    SignatureEnabled             bool                  `dynamodbav:"signatureEnabled"` // FAC for signature
    Keywords                     *string               `dynamodbav:"keywords,omitempty"`
    KeywordEnabled               bool                  `dynamodbav:"keywordEnabled"` // FAC for keywords
    ServiceRecommendation        *string               `dynamodbav:"serviceRecommendation,omitempty"`
    ServiceRecommendationEnabled bool                  `dynamodbav:"serviceRecommendationEnabled"` // FAC for serviceRecommendation
    AutoQuickReplyEnabled        bool                  `dynamodbav:"autoQuickReplyEnabled"`        // FAC for auto quick reply
}

func NewBusiness(lineUserId string,
    lineUserProfile linebot.UserProfileResponse,
    createdAt time.Time) User {
    user := User{
        UserId:                       lineUserId,
        SubscriptionTier:             enum.SubscriptionTierBeta,
        LineUsername:                 lineUserProfile.DisplayName,
        LineProfilePictureUrl:        &lineUserProfile.PictureURL,
        Language:                     &lineUserProfile.Language,
        CreatedAt:                    createdAt,
        LastUpdated:                  createdAt,
        EmojiEnabled:                 false,
        SignatureEnabled:             false,
        KeywordEnabled:               false,
        ServiceRecommendationEnabled: false,
        AutoQuickReplyEnabled:        false,
    }

    return user
}

func BuildDdbBusinessKey(userId string) map[string]*dynamodb.AttributeValue {
    uniqueId := util.DefaultUniqueId
    return map[string]*dynamodb.AttributeValue{
        "userId": {
            S: &userId,
        },
        "uniqueId": {
            S: &uniqueId,
        }}
}
