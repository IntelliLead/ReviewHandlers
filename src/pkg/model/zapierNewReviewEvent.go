package model

import (
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/go-playground/validator/v10"
    "regexp"
    "time"
)

type ZapierNewReviewEvent struct {
    // For legacy users that have not completed OAUTH
    UserId *string `json:"userId" `

    CreatedAt            time.Time          `json:"createdAt"`
    NumberRating         _type.NumberRating `json:"numberRating" validate:"min=1,max=5"`
    Review               *string            `json:"review"`
    ReviewLastUpdated    time.Time          `json:"reviewLastUpdated"`
    ReviewerName         string             `json:"reviewerName"`
    ReviewerProfilePhoto string             `json:"reviewerProfilePhoto" validate:"url"`
    VendorEventId        string             `json:"vendorEventId"`
    VendorReviewId       string             `json:"vendorReviewId" validate:"vendorReviewId"`
    LastReplied          *time.Time         `json:"lastReplied" validate:"required_with=Reply"` // optional
    Reply                *string            `json:"reply" validate:"required_with=LastReplied"` // optional
    ZapierReplyWebhook   string             `dynamodbav:"zapierReplyWebhook" validate:"url"`
}

// Regular expression for VendorReviewId
var vendorReviewIdRegex = regexp.MustCompile(`^accounts/\d+/locations/\d+/reviews/.+$`)

// Custom validation function for VendorReviewId
func validateVendorReviewId(fl validator.FieldLevel) bool {
    return vendorReviewIdRegex.MatchString(fl.Field().String())
}

func init() {
    validate := validator.New()
    err := validate.RegisterValidation("vendorReviewId", validateVendorReviewId)
    if err != nil {
        panic(err)
    }
}
