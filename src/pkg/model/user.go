package model

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "github.com/go-playground/validator/v10"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "sort"
    "strings"
    "time"
)

type User struct {
    UserId                       string                 `dynamodbav:"userId" validate:"required"`            // partition key
    ActiveBusinessId             bid.BusinessId         `dynamodbav:"activeBusinessId"  validate:"required"` // active business is the business that the user is currently managing. if len(BusinessIds) == 1, this field is BusinessIds[0]
    BusinessIds                  []bid.BusinessId       `dynamodbav:"businessIds,stringset,omitemptyelem" validate:"required,min=1"`
    CreatedAt                    time.Time              `dynamodbav:"createdAt,unixtime"  validate:"required"`
    LineUsername                 string                 `dynamodbav:"lineUsername"  validate:"required"`
    LineProfilePictureUrl        string                 `dynamodbav:"lineProfilePicture" validate:"required,url"`
    Language                     string                 `dynamodbav:"language"  validate:"required"`
    SubscriptionTier             *enum.SubscriptionTier `dynamodbav:"subscriptionTier,omitempty"`
    ExpireAt                     *time.Time             `dynamodbav:"expireAt,omitempty,unixtime"`
    LastUpdated                  time.Time              `dynamodbav:"lastUpdated,unixtime"  validate:"required"`
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

var validate *validator.Validate

func init() {
    validate = validator.New(validator.WithRequiredStructEnabled())
}

func NewUser(lineUserId string,
    businessIds []bid.BusinessId,
    lineUserProfile linebot.UserProfileResponse,
    google Google,
) (User, error) {
    if len(businessIds) == 0 {
        return User{}, errors.New("businessIds must not be empty")
    }
    user := User{
        UserId:                       lineUserId,
        ActiveBusinessId:             businessIds[0],
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

    err := validate.Struct(user)
    if err != nil {
        return User{}, errors.New("invalid user: " + err.Error())
    }

    return user, nil
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

func (u User) GetBusinessIdFromIndex(businessIdIndex int) (bid.BusinessId, error) {
    if businessIdIndex < 0 || businessIdIndex >= len(u.BusinessIds) {
        return "", fmt.Errorf("invalid businessIdIndex: %d", businessIdIndex)
    }

    sort.Slice(u.BusinessIds, func(i, j int) bool {
        return u.BusinessIds[i].String() < u.BusinessIds[j].String()
    })

    return u.BusinessIds[businessIdIndex], nil
}

func (u User) GetBusinessIdIndex(businessId bid.BusinessId) (int, error) {
    sort.Slice(u.BusinessIds, func(i, j int) bool {
        return u.BusinessIds[i].String() < u.BusinessIds[j].String()
    })

    idx := util.FindStringIndex(bid.BusinessIdsToStringSlice(u.BusinessIds), businessId.String())
    if idx == -1 {
        return -1, fmt.Errorf("businessId %s not found in user %s", businessId, u.UserId)
    }

    return idx, nil
}

// GetSortedBusinessIds returns the sorted businessIds of a user.
// businessIdIndex is reflected in the order of the returned businessIds.
func (u User) GetSortedBusinessIds() []bid.BusinessId {
    sort.Slice(u.BusinessIds, func(i, j int) bool {
        return u.BusinessIds[i].String() < u.BusinessIds[j].String()
    })

    return u.BusinessIds
}
