package bid

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
)

type BusinessId string

func NewBusinessId(businessIdStr string) (BusinessId, error) {
    if util.IsNumericString(businessIdStr) == false {
        return "", fmt.Errorf("BusinessId must be numeric string. '%s' is invalid", businessIdStr)
    }
    businessId := BusinessId(businessIdStr)

    return businessId, nil
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
