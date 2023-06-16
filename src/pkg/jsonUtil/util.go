package jsonUtil

import (
    "encoding/json"
)

func AnyToJson(obj any) string {
    return string(AnyToJsonObject(obj))
}

func AnyToJsonObject(obj any) []byte {
    // Convert the Person object to JSON
    jsonData, _ := json.Marshal(obj)
    return jsonData
}

func JsonToMap(jsonObj []byte) (map[string]interface{}, error) {
    var result map[string]interface{}
    err := json.Unmarshal(jsonObj, &result)
    if err != nil {
        return nil, err
    }
    return result, nil
}
