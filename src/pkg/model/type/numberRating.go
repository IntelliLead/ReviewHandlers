package _type

import (
    "encoding/json"
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "strconv"
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

func (nr *NumberRating) LineFlexTemplateJson() ([]interface{}, error) {
    n := int(*nr)
    if n < 1 || n > 5 {
        return nil, errors.New("invalid numberRating value: " + strconv.Itoa(n))
    }

    jsons := jsonUtil.LoadReviewMessageLineFlexTemplateJsons()

    goldStarJson, err := jsonUtil.JsonToMap(jsons.GoldStarIcon)
    if err != nil {
        return nil, err
    }

    grayStarJson, err := jsonUtil.JsonToMap(jsons.GrayStarIcon)
    if err != nil {
        return nil, err
    }

    stars := make([]interface{}, 5)

    for i := 0; i < 5; i++ {
        if i < n {
            // Use goldStarJson for filled stars
            stars[i] = goldStarJson
        } else {
            // Use grayStarJson for empty stars
            stars[i] = grayStarJson
        }
    }

    return stars, nil
}
