package util

import (
    "encoding/json"
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

func RemoveStringFromSlice(slice []string, str string) []string {
    var result []string
    for _, s := range slice {
        if s != str {
            result = append(result, s)
        }
    }
    return result
}

// FindStringIndex returns the first index of the target string, or -1 if no match is found.
func FindStringIndex(slice []string, target string) int {
    for i, value := range slice {
        if value == target {
            return i
        }
    }
    return -1 // not found
}

func DeepCopy(src interface{}) (interface{}, error) {
    // Marshal the source into JSON
    jsonObj, err := json.Marshal(src)
    if err != nil {
        return nil, err
    }

    // Unmarshal JSON into a new variable
    var dst interface{}
    err = json.Unmarshal(jsonObj, &dst)
    if err != nil {
        return nil, err
    }

    return dst, nil
}
