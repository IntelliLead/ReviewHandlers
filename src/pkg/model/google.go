package model

import (
    "time"
)

type Google struct {
    Id                  string    `dynamodbav:"id"`
    AccessToken         string    `dynamodbav:"accessToken"`
    AccessTokenExpireAt time.Time `dynamodbav:"accessTokenExpireAt"` // in nanoseconds precision. Stored as string in DDB
    RefreshToken        string    `dynamodbav:"refreshToken"`
    ProfileFullName     string    `dynamodbav:"profileFullName"`
    Email               string    `dynamodbav:"email" validate:"email"`
    ImageUrl            string    `dynamodbav:"imageUrl" validate:"url"`
    Locale              string    `dynamodbav:"locale"`
    BusinessAccountId   string    `dynamodbav:"businessAccountId" validate:"numeric"`
}
