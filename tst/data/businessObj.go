package data

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "time"
)

var (
    expiryTime, _ = time.Parse(time.RFC3339Nano, "2023-08-23T13:35:46.680761359Z")
    TestBusiness  = model.Business{
        BusinessId:     "accounts/106775638291982182570/locations/12251512170589559833",
        BusinessName:   "智引力 IntelliLead",
        UserIds:        []string{"Ucc29292b212e271132cee980c58e94eb"},
        KeywordEnabled: false,
        CreatedAt:      time.Unix(1692791310, 0), // Unix timestamp to time.Time conversion
        LastUpdated:    time.Unix(1692791310, 0),
        LastUpdatedBy:  "Ucc29292b212e271132cee980c58e94eb",
)
