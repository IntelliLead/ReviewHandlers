package googleUtil

import (
    "fmt"
    "strings"
    "time"
)

type GoogleReviewsResponse struct {
    Reviews          []GoogleReview `json:"reviews"`
    NextPageToken    string         `json:"nextPageToken"`
    TotalReviewCount int            `json:"totalReviewCount"`
    AverageRating    float64        `json:"averageRating"`
}

type GoogleReview struct {
    UpdateTime  time.Time    `json:"updateTime"`
    Name        string       `json:"name"`
    ReviewId    string       `json:"reviewId"`
    StarRating  string       `json:"starRating"`
    Reviewer    Reviewer     `json:"reviewer"`
    CreateTime  time.Time    `json:"createTime"`
    Comment     *string      `json:"comment,omitempty"` // omitempty because comment is not always present
    ReviewReply *ReviewReply `json:"reviewReply,omitempty"`
}

type Reviewer struct {
    ProfilePhotoUrl string `json:"profilePhotoUrl"`
    DisplayName     string `json:"displayName"`
}

type ReviewReply struct {
    Comment    string    `json:"comment"`
    UpdateTime time.Time `json:"updateTime"`
}

// Function to map star rating from string to integer
func StarRatingToInt(rating string) int {
    switch rating {
    case "ONE":
        return 1
    case "TWO":
        return 2
    case "THREE":
        return 3
    case "FOUR":
        return 4
    case "FIVE":
        return 5
    default:
        return 0 // Default case if the rating is not recognized
    }
}

type PerformanceMetric struct {
    Date    time.Time           `json:"date"`
    Metrics map[DailyMetric]int `json:"metrics"`
}

type DailyMetric int

const (
    DailyMetricBusinessImpressionsDesktopMaps DailyMetric = iota
    DailyMetricBusinessImpressionsDesktopSearch
    DailyMetricBusinessImpressionsMobileMaps
    DailyMetricBusinessImpressionsMobileSearch
    DailyMetricBusinessConversations
    DailyMetricBusinessDirectionRequests
    DailyMetricCallClicks
    DailyMetricWebsiteClicks
    DailyMetricBusinessBookings
    DailyMetricBusinessFoodOrders
    DailyMetricBusinessFoodMenuClicks
    DailyMetricUnknown
)

var dailyMetricStrings = map[DailyMetric]string{
    DailyMetricBusinessImpressionsDesktopMaps:   "BUSINESS_IMPRESSIONS_DESKTOP_MAPS",
    DailyMetricBusinessImpressionsDesktopSearch: "BUSINESS_IMPRESSIONS_DESKTOP_SEARCH",
    DailyMetricBusinessImpressionsMobileMaps:    "BUSINESS_IMPRESSIONS_MOBILE_MAPS",
    DailyMetricBusinessImpressionsMobileSearch:  "BUSINESS_IMPRESSIONS_MOBILE_SEARCH",
    DailyMetricBusinessConversations:            "BUSINESS_CONVERSATIONS",
    DailyMetricBusinessDirectionRequests:        "BUSINESS_DIRECTION_REQUESTS",
    DailyMetricCallClicks:                       "CALL_CLICKS",
    DailyMetricWebsiteClicks:                    "WEBSITE_CLICKS",
    DailyMetricBusinessBookings:                 "BUSINESS_BOOKINGS",
    DailyMetricBusinessFoodOrders:               "BUSINESS_FOOD_ORDERS",
    DailyMetricBusinessFoodMenuClicks:           "BUSINESS_FOOD_MENU_CLICKS",
    DailyMetricUnknown:                          "DAILY_METRIC_UNKNOWN",
}

var stringToDailyMetric = make(map[string]DailyMetric)

func init() {
    for k, v := range dailyMetricStrings {
        stringToDailyMetric[v] = k
    }
}

func (s DailyMetric) String() string {
    return dailyMetricStrings[s]
}

func (s DailyMetric) Strings() []string {
    var metrics []string
    for k, v := range dailyMetricStrings {
        if k != DailyMetricUnknown {
            metrics = append(metrics, v)
        }
    }
    return metrics
}

func ToDailyMetric(str string) (DailyMetric, error) {
    if val, ok := stringToDailyMetric[strings.ToUpper(str)]; ok {
        return val, nil
    }
    return DailyMetricUnknown, fmt.Errorf("invalid daily metric: %s", str)
}
