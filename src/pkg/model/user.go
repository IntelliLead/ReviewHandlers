package model

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "strings"
    "time"
)

type User struct {
    UserId string `dynamodbav:"userId"` // partition key
    // TODO: [INT-91] Remove backfill logic once all users have been backfilled
    ActiveBusinessId             *string                `dynamodbav:"activeBusinessId,omitempty"`
    BusinessIds                  []string               `dynamodbav:"businessIds,stringset,omitemptyelem" validate:"min=1"`
    CreatedAt                    time.Time              `dynamodbav:"createdAt,unixtime"`
    LineUsername                 string                 `dynamodbav:"lineUsername"`
    LineProfilePictureUrl        string                 `dynamodbav:"lineProfilePicture" validate:"url"`
    Language                     string                 `dynamodbav:"language"`
    SubscriptionTier             *enum.SubscriptionTier `dynamodbav:"subscriptionTier,omitempty"`
    ExpireAt                     *time.Time             `dynamodbav:"expireAt,omitempty,unixtime"`
    LastUpdated                  time.Time              `dynamodbav:"lastUpdated,unixtime"`
    QuickReplyMessage            *string                `dynamodbav:"quickReplyMessage,omitempty"`
    BusinessDescription          *string                `dynamodbav:"businessDescription,omitempty"` // TODO: [INT-91] remove this field
    EmojiEnabled                 bool                   `dynamodbav:"emojiEnabled"`                  // FAC for emoji
    Signature                    *string                `dynamodbav:"signature,omitempty"`
    SignatureEnabled             bool                   `dynamodbav:"signatureEnabled"`   // FAC for signature
    Keywords                     *string                `dynamodbav:"keywords,omitempty"` // TODO: [INT-91] remove this field
    KeywordEnabled               *bool                  `dynamodbav:"keywordEnabled"`     // FAC for keywords    // TODO: [INT-91] remove this field
    ServiceRecommendation        *string                `dynamodbav:"serviceRecommendation,omitempty"`
    ServiceRecommendationEnabled bool                   `dynamodbav:"serviceRecommendationEnabled"` // FAC for serviceRecommendation
    AutoQuickReplyEnabled        *bool                  `dynamodbav:"autoQuickReplyEnabled"`        // FAC for auto quick reply // TODO: [INT-91] remove this field
    Google                       Google                 `dynamodbav:"google,omitemptyelem"`
}

func NewUser(lineUserId string,
    businessIds []string,
    lineUserProfile linebot.UserProfileResponse,
    google Google,
) User {
    user := User{
        UserId:                       lineUserId,
        ActiveBusinessId:             &businessIds[0],
        BusinessIds:                  businessIds,
        LineUsername:                 lineUserProfile.DisplayName,
        LineProfilePictureUrl:        lineUserProfile.PictureURL,
        Language:                     lineUserProfile.Language,
        CreatedAt:                    time.Now(),
        LastUpdated:                  time.Now(),
        EmojiEnabled:                 false,
        SignatureEnabled:             false,
        ServiceRecommendationEnabled: false,
        Google:                       google,
    }

    return user
}

func BuildDdbUserKey(userId string) map[string]types.AttributeValue {
    uniqueId := util.DefaultUniqueId
    return map[string]types.AttributeValue{
        "userId":   &types.AttributeValueMemberS{Value: userId},
        "uniqueId": &types.AttributeValueMemberS{Value: uniqueId},
    }
}

// GetFinalQuickReplyMessage returns the final quick reply message to be sent to the user.
// It replaces the {評論人} placeholder with the reviewer's name.
// TODO: [INT-97] Remove this helper when all users are backfilled with active business ID
func (u User) GetFinalQuickReplyMessage(review Review) string {
    if util.IsEmptyStringPtr(u.QuickReplyMessage) {
        return ""
    }

    return strings.ReplaceAll(*u.QuickReplyMessage, "{評論人}", review.ReviewerName)
}
