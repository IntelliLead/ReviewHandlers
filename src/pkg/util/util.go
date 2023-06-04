package util

import (
    "encoding/json"
    "strings"
    "time"
)

func AnyToJson(obj any) string {
    return string(AnyToJsonObject(obj))
}

func AnyToJsonObject(obj any) []byte {
    // Convert the Person object to JSON
    jsonData, _ := json.Marshal(obj)
    return jsonData
}

func UtcToReadableTwTimestamp(timestamp time.Time) (string, error) {
    loc, err := time.LoadLocation("Asia/Taipei")
    if err != nil {
        return "", err
    }
    taipeiTime := timestamp.In(loc)

    return taipeiTime.Format("2006.01.02 03:04:05 PM"), nil
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
