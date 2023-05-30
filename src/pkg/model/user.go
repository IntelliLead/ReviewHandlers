package model

import (
    "time"
)

type User struct {
	UserID    string    `dynamodbav:"userId"`    // partition key
	CreatedAt time.Time `dynamodbav:"createdAt"` // sort key

	// LineUserID       string           `dynamodbav:"lineUserId" json:"omitempty"`
	LineID           *string          `dynamodbav:"lineId"`
	SubscriptionTier SubscriptionTier `dynamodbav:"subscriptionTier"`
	ExpireAt         *time.Time       `dynamodbav:"expireAt"`
	LastUpdated time.Time `dynamodbav:"lastUpdated"`
}

func NewUser(lineUserId string, createdAt time.Time) User {
	user := User{
		UserID:           lineUserId,
		// LineUserID:       lineUserId,
		SubscriptionTier: SubscriptionTierBeta,
		CreatedAt:        createdAt,
		LastUpdated:      createdAt,
	}

	return user
}
