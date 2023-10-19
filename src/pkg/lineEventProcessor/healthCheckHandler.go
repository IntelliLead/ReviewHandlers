package lineEventProcessor

import (
    "encoding/json"
    "github.com/aws/aws-lambda-go/events"
    "go.uber.org/zap"
)

/*
HandleHealthCheck - The LINE Platform may send an HTTP POST request that doesn't include a webhook event to confirm communication. In this case, send a 200 status code.

Parameters:

	request - The request from the LINE Messaging webhook source

Returns:

	bool - true if the request is a health check call and was handled, false otherwise
*/
func HandleHealthCheck(request events.LambdaFunctionURLRequest, log *zap.SugaredLogger) (bool, error) {
    var body map[string]interface{}
    err := json.Unmarshal([]byte(request.Body), &body)
    if err != nil {
        log.Error("Error parsing request body:", err)
        return false, err
    }

    parsedEvents, ok := body["events"].([]interface{})
    if !ok || len(parsedEvents) == 0 {
        log.Info("Request doesn't include any events. Likely a health check call.")
        return true, nil
    }

    return false, nil
}
