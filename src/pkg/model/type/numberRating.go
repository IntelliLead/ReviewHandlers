package _type

import (
    "encoding/json"
    "fmt"
)

type NumberRating int

func (nr *NumberRating) UnmarshalJSON(data []byte) error {
    var rating string
    err := json.Unmarshal(data, &rating)
    if err != nil {
        return err
    }

    switch rating {
    case "1", "2", "3", "4", "5":
        *nr = NumberRating(rating[0] - '0')
    default:
        return fmt.Errorf("invalid numberRating value: %s", rating)
    }

    return nil
}
