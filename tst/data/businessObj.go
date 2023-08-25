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
        Google: &model.Google{
            Id:                  "106775638291982182570",
            AccessToken:         "ya29.a0AfB_byBmQ_8Kzict4JwNTjuErlrATV66W_w7jKSs7QuLZOou2rD-DjeQRfHr41UOR3vdprTWUUt-4qqoXTQ4ngj-HMTKfRsihmEEYVCv9KTjB1mL9SvUuGkw1E9JEUkrUvfr2t5JsmsWdO7xBwQoA_CSV6YPl7vJsVxzkwaCgYKAVUSARMSFQHsvYlsUla8aXVuPtDjP1Io6YglXQ0173",
            AccessTokenExpireAt: expiryTime,
            RefreshToken:        "1//0exB02Lys6-AyCgYIARAAGA4SNwF-L9IrPYA0Qvzln5IZBydMdjg0Z5HCX9uejJfyeya5ucsKEYXrBwoi2vShOUE31K3-1Xfd1Ok",
            ProfileFullName:     "Shawn Wang",
            Email:               "shawn@il-tw.com",
            ImageUrl:            "https://lh3.googleusercontent.com/a/AAcHTtdVU6pNMqVtSUiUDa3giXpDYFuKQRI",
            Locale:              "en",
        }}
)
