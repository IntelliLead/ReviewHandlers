package model

import (
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
    "time"
)

type ZapierNewReviewEvent struct {
    // at least 1 must be provided
    UserId     *string         `json:"userId" validate:"required_without=BusinessId"`
    BusinessId *bid.BusinessId `json:"businessId" validate:"required_without=UserId"`

    CreatedAt            time.Time          `json:"createdAt"`
    NumberRating         _type.NumberRating `json:"numberRating" validate:"min=1,max=5"`
    Review               *string            `json:"review"`
    ReviewLastUpdated    time.Time          `json:"reviewLastUpdated"`
    ReviewerName         string             `json:"reviewerName"`
    ReviewerProfilePhoto string             `json:"reviewerProfilePhoto" validate:"url"`
    VendorEventId        string             `json:"vendorEventId"`
    VendorReviewId       string             `json:"vendorReviewId"`
    LastReplied          *time.Time         `json:"lastReplied" validate:"required_with=Reply"` // optional
    Reply                *string            `json:"reply" validate:"required_with=LastReplied"` // optional
    ZapierReplyWebhook   string             `dynamodbav:"zapierReplyWebhook" validate:"url"`
}
