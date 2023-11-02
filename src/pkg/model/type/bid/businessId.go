package bid

import (
    "errors"
    "github.com/go-playground/validator/v10"
)

type BusinessId string

var (
    validate = validator.New(validator.WithRequiredStructEnabled())
)

func NewBusinessId(businessId string) (BusinessId, error) {
    bid := BusinessId(businessId)
    if err := bid.validate(); err != nil {
        return "", err
    }
    return bid, nil
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
