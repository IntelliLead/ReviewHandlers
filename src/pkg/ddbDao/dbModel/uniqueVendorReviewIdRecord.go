package dbModel

import "github.com/IntelliLead/ReviewHandlers/src/pkg/model"

type UniqueVendorReviewIdRecord struct {
    UserId                  string `dynamodbav:"userId"`   // partition key
    VendorReviewIdUniqueKey string `dynamodbav:"uniqueId"` // sort key
}

const uniqueVendorReviewIdPrefix = "#UNIQUE_VENDOR_REVIEW_ID#"

func NewUniqueVendorReviewIdRecord(review model.Review) UniqueVendorReviewIdRecord {
    uniqueVendorReviewID := UniqueVendorReviewIdRecord{
        UserId:                  review.UserId,
        VendorReviewIdUniqueKey: uniqueVendorReviewIdPrefix + review.VendorReviewId,
    }
    return uniqueVendorReviewID
}
