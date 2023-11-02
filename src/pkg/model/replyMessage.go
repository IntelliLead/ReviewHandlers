package model

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/urid"
)

type Reply struct {
    UserReviewId urid.UserReviewId
    Message      string `validate:"min=1"`
}

func NewReply(userReviewId urid.UserReviewId, message string) (Reply, error) {
    reply := Reply{
        UserReviewId: userReviewId,
        Message:      message,
    }

    return reply, nil
}
