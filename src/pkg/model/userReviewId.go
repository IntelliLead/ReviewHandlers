package model

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/rid"
    "github.com/go-playground/validator/v10"
    "strconv"
    "strings"
)

// UserReviewId holds the businessIdIndex and reviewId.
type UserReviewId struct {
    BusinessIdIndex *int `validate:"omitempty,min=0,max=500"`
    ReviewId        rid.ReviewId
}

var (
    validateUserReviewId = validator.New(validator.WithRequiredStructEnabled())
)

// NewUserReviewId creates a new UserReviewId with the given businessIdIndex and reviewId.
func NewUserReviewId(businessIdIndex *int, reviewId rid.ReviewId) (UserReviewId, error) {
    ur := UserReviewId{
        BusinessIdIndex: businessIdIndex,
        ReviewId:        reviewId,
    }

    // Validate the struct
    err := validateUserReviewId.Struct(ur)
    if err != nil {
        return UserReviewId{}, err
    }

    return ur, nil
}

// String returns the string representation of UserReviewId.
func (ur UserReviewId) String() string {
    if ur.BusinessIdIndex == nil {
        return ur.ReviewId.String()
    }

    return fmt.Sprintf("%d|%s", *ur.BusinessIdIndex, ur.ReviewId.String())
}

// ParseUserReviewId decodes a string into a UserReviewId.
func ParseUserReviewId(encoded string) (UserReviewId, error) {
    // TODO: [INT-97] remove backwards compatible logic after all users are authed and have businessIdIndex
    // if no separator, interpret whole string as reviewId
    if !strings.Contains(encoded, "|") {
        reviewId, err := rid.NewReviewId(encoded)
        if err != nil {
            return UserReviewId{}, err
        }
        return NewUserReviewId(nil, reviewId)
    } else {
        parts := strings.Split(encoded, "|")
        if len(parts) != 2 {
            return UserReviewId{}, errors.New("invalid UserReviewId: " + encoded)
        }

        var businessIdIndex *int
        if len(parts[0]) > 0 {
            index, err := strconv.Atoi(parts[0])
            if err != nil {
                return UserReviewId{}, errors.New("invalid businessIdIndex: " + parts[0])
            }
            businessIdIndex = &index
        }

        reviewId, err := rid.NewReviewId(parts[1])
        if err != nil {
            return UserReviewId{}, err
        }

        return NewUserReviewId(businessIdIndex, reviewId)
    }
}
