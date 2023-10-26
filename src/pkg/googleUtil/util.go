package googleUtil

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "golang.org/x/oauth2"
    "google.golang.org/api/mybusinessbusinessinformation/v1"
)

func GoogleToToken(g model.Google) oauth2.Token {
    return oauth2.Token{
        AccessToken:  g.AccessToken,
        TokenType:    "Bearer",
        RefreshToken: g.RefreshToken,
        Expiry:       g.AccessTokenExpireAt,
    }
}

func FilterOpenBusinessLocations(businessLocations []mybusinessbusinessinformation.Location) []mybusinessbusinessinformation.Location {
    openBusinessLocations := make([]mybusinessbusinessinformation.Location, 0)

    for _, location := range businessLocations {
        if location.OpenInfo.Status == "OPEN" || location.OpenInfo.Status == "OPEN_FOR_BUSINESS_UNSPECIFIED" {
            openBusinessLocations = append(openBusinessLocations, location)
        }
    }

    return openBusinessLocations
}

func MapBusinessIds(accountId string, businessLocations []mybusinessbusinessinformation.Location) []string {
    businessIds := make([]string, 0, len(businessLocations))
    for _, location := range businessLocations {
        businessIds = append(businessIds, accountId+"/"+location.Name)
    }

    return businessIds
}
