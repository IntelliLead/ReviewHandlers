package util

import "encoding/json"

func AnyToJson(obj any) string {
    // Convert the Person object to JSON
    jsonData, _ := json.Marshal(obj)
    return string(jsonData)
}