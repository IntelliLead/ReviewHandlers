package dbModel

import "github.com/IntelliLead/ReviewHandlers/src/pkg/model"

type UniqueVendorReviewIdRecord struct {
    BusinessId              string `dynamodbav:"userId"`   // partition key
    VendorReviewIdUniqueKey string `dynamodbav:"uniqueId"` // sort key
}

const uniqueVendorReviewIdPrefix = "#UNIQUE_VENDOR_REVIEW_ID#"

func NewUniqueVendorReviewIdRecord(review model.Review) UniqueVendorReviewIdRecord {
    uniqueVendorReviewID := UniqueVendorReviewIdRecord{
        BusinessId:              review.BusinessId,
        VendorReviewIdUniqueKey: uniqueVendorReviewIdPrefix + review.VendorReviewId,
    }
    return uniqueVendorReviewID
}
