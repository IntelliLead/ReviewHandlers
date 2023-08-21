package model

import "time"

type Google struct {
    Id                  string    `dynamodbav:"id"`
    AccessToken         string    `dynamodbav:"accessToken"`
    accessTokenExpireAt time.Time `dynamodbav:"accessTokenExpireAt,unixtime"`
    RefreshToken        string    `dynamodbav:"refreshToken"`
    ProfileFullName     string    `dynamodbav:"profileFullName"`
    Email               string    `dynamodbav:"email" validate:"email"`
    ImageUrl            string    `dynamodbav:"imageUrl" validate:"url"`
}