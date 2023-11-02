package urid

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/rid"
    "strconv"
    "strings"
)

// See design: https://www.notion.so/intellilead/Review-ID-in-user-s-reply-ccae5f05232a406cb079e00eb99503dd?pvs=4
type UserReviewId string

func NewUserReviewId(businessIdIndex int, reviewId rid.ReviewId) string {
    return fmt.Sprintf("%d|%s", businessIdIndex, reviewId.String())
}

// Decode returns the businessIdIndex and reviewId of a UserReviewId
func (ur UserReviewId) Decode() (int, rid.ReviewId, error) {
    parts := strings.Split(string(ur), "|")
    if len(parts) != 2 {
        return 0, "", errors.New("invalid UserReviewId: " + string(ur))
    }
    if len(parts[0]) == 0 {
        return 0, "", errors.New("businessIdIndex is empty")
    }
    if len(parts[1]) == 0 {
        return 0, "", errors.New("reviewId is empty")
    }

    businessIdIndex, err := strconv.Atoi(parts[0])
    if err != nil {
        return 0, "", errors.New("invalid businessIdIndex: " + parts[0])
    }
    reviewId, err := rid.NewReviewId(parts[1])
    if err != nil {
        return 0, "", err
    }

    return businessIdIndex, reviewId, nil
}

func (ur UserReviewId) String() string {
    return string(ur)
}
