package model

type Reply struct {
    UserReviewId UserReviewId
    Message      string `validate:"min=1"`
}

func NewReply(userReviewId UserReviewId, message string) (Reply, error) {
    reply := Reply{
        UserReviewId: userReviewId,
        Message:      message,
    }

    return reply, nil
}
