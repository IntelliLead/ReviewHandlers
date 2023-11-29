package googleUtil

import (
    "bytes"
    "encoding/csv"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "golang.org/x/oauth2"
    "google.golang.org/api/mybusinessbusinessinformation/v1"
    "strconv"
    "time"
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

// GoogleReviewsToCSV converts a slice of GoogleReview structs to a CSV file, omitting ProfilePhotoUrl field for brevity
// Returns a pointer to a bytes.Buffer containing the CSV file
func GoogleReviewsToCSV(reviews []GoogleReview) (*bytes.Buffer, error) {

    // file, err := os.Create(filePath)
    // if err != nil {
    //     return err
    // }
    // defer file.Close()
    // writer := csv.NewWriter(file)

    buffer := &bytes.Buffer{}
    writer := csv.NewWriter(buffer)

    header := []string{"UpdateTime", "Name", "ReviewId", "StarRating", "ReviewerName", "CreateTime", "Comment", "ReviewReplyComment", "ReviewReplyUpdateTime"}
    if err := writer.Write(header); err != nil {
        return nil, err
    }

    for _, review := range reviews {
        var comment, replyComment string
        var replyUpdateTime string

        if !util.IsEmptyStringPtr(review.Comment) {
            comment = fmt.Sprintf("\"%s\"", util.ReplaceNewLines(removeGoogleTranslate(*review.Comment)))
        }

        if review.ReviewReply != nil {
            replyComment = fmt.Sprintf("\"%s\"", util.ReplaceNewLines(removeGoogleTranslate(review.ReviewReply.Comment)))
            replyUpdateTime = review.ReviewReply.UpdateTime.Format(time.RFC3339)
        }

        record := []string{
            review.UpdateTime.Format(time.RFC3339),
            review.Name,
            review.ReviewId,
            strconv.Itoa(StarRatingToInt(review.StarRating)),
            review.Reviewer.DisplayName,
            review.CreateTime.Format(time.RFC3339),
            comment,
            replyComment,
            replyUpdateTime,
        }

        if err := writer.Write(record); err != nil {
            return nil, err
        }
    }

    writer.Flush()
    return buffer, writer.Error()
}

func removeGoogleTranslate(comment string) string {
    originalLine, translationFound := util.ExtractOriginalFromGoogleTranslate(comment)
    if translationFound {
        return originalLine
    }

    return comment
}

// PerformanceMetricsToCSV converts a slice of PerformanceMetric structs to a CSV file
// Returns a pointer to a bytes.Buffer containing the CSV file
func PerformanceMetricsToCSV(metrics []PerformanceMetric) (*bytes.Buffer, error) {
    buffer := &bytes.Buffer{}
    writer := csv.NewWriter(buffer)

    // Write the header
    header := []string{"Date"}
    for _, dailyMetricStr := range DailyMetric.Strings(0) {
        header = append(header, dailyMetricStr)
    }
    if err := writer.Write(header); err != nil {
        return nil, err
    }

    // Write the records
    for _, metric := range metrics {
        record := make([]string, len(header))
        record[0] = metric.Date.Format(time.RFC3339)

        for i, dailyMetricStr := range header {
            if i == 0 {
                continue
            }

            dailyMetric, err := ToDailyMetric(dailyMetricStr)
            if err != nil {
                return nil, err
            }

            value, exists := metric.Metrics[dailyMetric]
            if exists {
                record[i] = strconv.Itoa(value)
            } else {
                record[i] = "NaN" // Default value if metric does not exist
            }
        }

        if err := writer.Write(record); err != nil {
            return nil, err
        }
    }

    writer.Flush()
    return buffer, writer.Error()
}
