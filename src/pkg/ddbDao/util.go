package ddbDao

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "golang.org/x/oauth2"
)

const KeyNotExistsConditionExpression = "attribute_not_exists(userId) AND attribute_not_exists(uniqueId)"

func GetToken(business model.Business) oauth2.Token {
    return oauth2.Token{
        AccessToken:  business.Google.AccessToken,
        RefreshToken: business.Google.RefreshToken,
        TokenType:    "Bearer",
        Expiry:       business.Google.AccessTokenExpireAt,
    }
}
