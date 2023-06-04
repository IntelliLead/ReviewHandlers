package model

import (
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "time"
)

type ZapierNewReviewEvent struct {
    CreatedAt            time.Time          `json:"createdAt"`
    NumberRating         _type.NumberRating `json:"numberRating" validate:"min=1,max=5"`
    Review               string             `json:"review"`
    ReviewLastUpdated    time.Time          `json:"reviewLastUpdated"`
    ReviewerName         string             `json:"reviewerName"`
    ReviewerProfilePhoto string             `json:"reviewerProfilePhoto" validate:"url"`
    VendorEventId        string             `json:"vendorEventId"`
    VendorReviewId       string             `json:"vendorReviewId"`
    UserId               string             `json:"userId"`
    LastReplied          *time.Time         `json:"lastReplied" validate:"required_with=Reply"` // optional
    Reply                *string            `json:"reply" validate:"required_with=LastReplied"` // optional
    // TODO: remove https://linear.app/vest/issue/INT-23/each-zapier-webhook-url-is-unique-to-the-user
    ZapierReplyWebhook string `dynamodbav:"zapierReplyWebhook" validate:"url"`
}
