package model

type ReplyToZapierEvent struct {
    VendorReviewId string `json:"vendorReviewId"`
    Message        string `json:"message" `
}
