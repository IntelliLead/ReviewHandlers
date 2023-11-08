package bid

import (
    "errors"
    "github.com/go-playground/validator/v10"
    "regexp"
)

type BusinessId string

var (
    validate        = validator.New(validator.WithRequiredStructEnabled())
    businessIdRegex = regexp.MustCompile(`^accounts/\d+/locations/\d+$`)
)

func init() {
    err := validate.RegisterValidation("businessId", validateBusinessId)
    if err != nil {
        panic(err)
    }
}

func NewBusinessId(businessIdStr string) (BusinessId, error) {
    businessId := BusinessId(businessIdStr)
    if err := validate.Var(businessId, "businessId"); err != nil {
        return "", errors.New("invalid BusinessId format")
    }

    return businessId, nil
}

func (bid BusinessId) validate() error {
    err := validate.Var(string(bid), "matches=^accounts/\\d+/locations/\\d+$")
    if err != nil {
        return errors.New("invalid BusinessId format")
    }
    return nil
}

func (bid BusinessId) String() string {
    return string(bid)
}

func IsValidBusinessId(data string) bool {
    _, err := NewBusinessId(data)
    return err == nil
}

func BusinessIdsToStringSlice(bids []BusinessId) []string {
    businessIdsStr := make([]string, len(bids))
    for i, v := range bids {
        businessIdsStr[i] = string(v)
    }
    return businessIdsStr
}

func validateBusinessId(fl validator.FieldLevel) bool {
    return businessIdRegex.MatchString(fl.Field().String())
}
