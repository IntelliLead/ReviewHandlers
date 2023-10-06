package model

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/go-playground/validator/v10"
    "time"
)

type Review struct {
    BusinessId           string `dynamodbav:"userId"` // partition key. It used to be userId, but we have since changed it to businessId without modifying the DDB table. See here for why: https://linear.app/intellilead/issue/INT-86/refactor-and-backfill-review-table
    UserId               string
    ReviewId             *_type.ReviewId    `dynamodbav:"uniqueId" validate:"omitempty,reviewIdValidation"` // sort key, to be generated by DAO. Not optional during DDB interaction
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

func NewReview(businessId string, event ZapierNewReviewEvent) (*Review, error) {
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
        ZapierReplyWebhook:   event.ZapierReplyWebhook, // TODO: retrieve from User DB https://linear.app/vest/issue/INT-23/each-zapier-webhook-url-is-unique-to-the-user
    }

    validate := validator.New()
    // even with omitEmpty and reviewId is indeed empty, validation registration is still needed
    err := validate.RegisterValidation("reviewIdValidation", _type.ReviewIdPtrValidation)
    if err != nil {
        return nil, err
    }
    err = validate.Struct(review)
    if err != nil {
        return nil, err
    }

    return &review, nil
}

func ValidateReview(review *Review) error {
    validate := validator.New()
    // even with omitEmpty and reviewId is indeed empty, validation registration is still needed
    err := validate.RegisterValidation("reviewIdValidation", _type.ReviewIdValidation)
    if err != nil {
        return err
    }
    return validate.Struct(review)
}
