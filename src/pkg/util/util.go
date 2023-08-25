package util

import (
    "strings"
    "time"
)

func UtcToReadableTwTimestamp(timestamp time.Time) (string, error) {
    loc, err := time.LoadLocation("Asia/Taipei")
    if err != nil {
        return "", err
    }
    taipeiTime := timestamp.In(loc)

    return taipeiTime.Format("2006-1-02 3:04:05PM"), nil
}

// ExtractOriginalFromGoogleTranslate extracts the original text from Google Translate
// Example
// `
// (Translated by Google) A local technology company in Taoyuan!
//
// (Original)
// 桃園當地科技公司誒！
// `
// returns
// 桃園當地科技公司誒！
func ExtractOriginalFromGoogleTranslate(text string) (originalLines string, found bool) {
    lines := strings.Split(text, "\n")
    for i, line := range lines {
        if strings.TrimSpace(line) == "(Original)" {
            found = true
            if i+1 < len(lines) {
                originalLines = strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
            }
            break
        }
    }
    return originalLines, found
}

func IsEmptyString(s string) bool {
    return len(strings.TrimSpace(s)) == 0
}

func IsEmptyStringPtr(s *string) bool {
    return s == nil || len(strings.TrimSpace(*s)) == 0
}

func GetToggleUrl(state bool) string {
    if state {
        return ToggleOnFlexMessageImageUrl
    }
    return ToggleOffFlexMessageImageUrl
}

func StringInSlice(str string, list []string) bool {
    for _, v := range list {
        if v == str {
            return true
        }
    }
    return false
}
