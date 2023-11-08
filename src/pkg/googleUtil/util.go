package googleUtil

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
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

func MapBusinessIds(accountId string, businessLocations []mybusinessbusinessinformation.Location) ([]bid.BusinessId, error) {
    businessIds := make([]bid.BusinessId, 0, len(businessLocations))
    for _, location := range businessLocations {
        businessId, err := bid.NewBusinessId(accountId + "/" + location.Name)
        if err != nil {
            return []bid.BusinessId{}, err
        }
        businessIds = append(businessIds, businessId)
    }

    return businessIds, nil
}
