package _type

import (
    "encoding/json"
    "fmt"
    "strings"
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

// String returns a star representation of the number rating.
// 1 star: ★☆☆☆☆
// 2 stars: ★★☆☆☆
// 3 stars: ★★★☆☆
// 4 stars: ★★★★☆
// 5 stars: ★★★★★
func (nr *NumberRating) String() string {
    n := int(*nr)
    if n < 1 || n > 5 {
        return ""
    }

    // ⭐ White Medium Star U+2B50 looks better because can be rendered as emoji, but unfortunately the hollow star emoji does not exist
    solidStar := "\u2605"
    hollowStar := "\u2606"

    whiteStars := strings.Repeat(solidStar, n)
    blackStars := strings.Repeat(hollowStar, 5-n)

    return whiteStars + blackStars
}
