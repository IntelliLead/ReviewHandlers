package model

import (
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/go-playground/validator/v10"
)

type Reply struct {
    ReviewId _type.ReviewId `validate:"reviewIdValidation"`
    Message  string         `validate:"min=1"`
}

func NewReply(reviewId _type.ReviewId, message string) (Reply, error) {
    validate := validator.New()
    // even with omitEmpty and reviewId is indeed empty, validation registration is still needed
    err := validate.RegisterValidation("reviewIdValidation", _type.ReviewIdValidation)
    if err != nil {
        return Reply{}, err
    }

    reply := Reply{
        ReviewId: reviewId,
        Message:  message,
    }

    err = validate.Struct(reply)
    if err != nil {
        return Reply{}, err
    }

    return reply, nil
}
