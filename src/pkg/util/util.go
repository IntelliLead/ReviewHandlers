package util

import (
    "encoding/json"
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