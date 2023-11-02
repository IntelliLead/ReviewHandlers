package model

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/rid"
    "github.com/go-playground/validator/v10"
    "time"
)

type Review struct {
    BusinessId           string `dynamodbav:"userId"` // partition key. It used to be userId, but we have since changed it to businessId without modifying the DDB table. See here for why: https://linear.app/intellilead/issue/INT-86/refactor-and-backfill-review-table
    UserId               string
    ReviewId             rid.ReviewId       `dynamodbav:"uniqueId" validate:"reviewIdValidation"` // sort key
    ZapierReplyWebhook   string             `dynamodbav:"zapierReplyWebhook" validate:"url"`
    VendorReviewId       string             `dynamodbav:"vendorReviewId"` // the businessId encoded here is from onboarding@tryintellilead.com, which is different from the one in the partition key (obtained via OAUTH)
    VendorEventId        string             `dynamodbav:"vendorEventId"`
    NumberRating         _type.NumberRating `dynamodbav:"numberRating" validate:"min=1,max=5"`
    Review               *string            `dynamodbav:"review,omitempty"`
    CreatedAt            time.Time          `dynamodbav:"createdAt,unixtime"`
    ReviewLastUpdated    time.Time          `dynamodbav:"reviewLastUpdated,unixtime"`
    ReviewerProfilePhoto string             `dynamodbav:"reviewerProfilePhoto" validate:"url"`
    ReviewerName         string             `dynamodbav:"reviewerName"`
    Reply                *string            `dynamodbav:"reply,omitempty" validate:"required_with=LastReplied"`          // optional
    LastReplied          *time.Time         `dynamodbav:"lastReplied,omitempty,unixtime" validate:"required_with=Reply"` // optional
    LastUpdated          time.Time          `dynamodbav:"lastUpdated,unixtime"`
    Vendor               enum.Vendor        `dynamodbav:"vendor"`
}

var reviewIdValidate *validator.Validate

func init() {
    // Initialize the validator instance
    reviewIdValidate = validator.New(validator.WithRequiredStructEnabled())
    // Register custom validations if needed
    _ = reviewIdValidate.RegisterValidation("reviewIdValidation", rid.ReviewIdPtrNumericValidation)
}

func NewReview(businessId string,
    reviewId rid.ReviewId,
    event ZapierNewReviewEvent) (Review, error) {
    var replyCopy *string = nil
    if event.Reply != nil {
        *replyCopy = *event.Reply
    }

    var lastRepliedCopy *time.Time = nil
    if event.LastReplied != nil {
        *lastRepliedCopy = *event.LastReplied
    }

    review := Review{
        BusinessId:           businessId,
        UserId:               event.UserId, // TODO: [INT-91] remove legacy logic once all users have been backfilled
        ReviewId:             reviewId,
        VendorReviewId:       event.VendorReviewId,
        VendorEventId:        event.VendorEventId,
        NumberRating:         event.NumberRating,
        Review:               event.Review,
        CreatedAt:            event.CreatedAt,
        ReviewLastUpdated:    event.ReviewLastUpdated,
        ReviewerProfilePhoto: event.ReviewerProfilePhoto,
        ReviewerName:         event.ReviewerName,
        Reply:                replyCopy,
        LastReplied:          lastRepliedCopy,
        LastUpdated:          time.Now(),
        Vendor:               enum.VendorGoogle,
        ZapierReplyWebhook:   event.ZapierReplyWebhook,
    }

    err := reviewIdValidate.Struct(review)
    if err != nil {
        return Review{}, err
    }

    return review, nil
}

func ValidateReview(review *Review) error {
    return reviewIdValidate.Struct(review)
}
