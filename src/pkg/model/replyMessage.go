package model

import (
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/go-playground/validator/v10"
)

type ReplyMessage struct {
    ReviewId _type.ReviewId `validate:"reviewIdValidation"`
    Message  string         `validate:"min=1"`
}

func NewReplyMessage(reviewId _type.ReviewId, message string) (ReplyMessage, error) {
    validate := validator.New()
    // even with omitEmpty and reviewId is indeed empty, validation registration is still needed
    err := validate.RegisterValidation("reviewIdValidation", _type.ReviewIdValidation)
    if err != nil {
        return ReplyMessage{}, err
    }

    replyMessage := ReplyMessage{
        ReviewId: reviewId,
        Message:  message,
    }

    err = validate.Struct(replyMessage)
    if err != nil {
        return ReplyMessage{}, err
    }

    return replyMessage, nil
}
