package zapierUtil

import (
    "bytes"
    "encoding/json"
    "go.uber.org/zap"
    "net/http"
)

type Zapier struct {
    log *zap.SugaredLogger
}

func NewZapier(logger *zap.SugaredLogger) *Zapier {
    return &Zapier{
        log: logger,
    }
}

func (z *Zapier) SendReplyEvent(webhookUrl string, payload ReplyToZapierEvent) error {
    // Convert the payload object to JSON
    // zapier expects array JSON payload
    jsonData, err := json.Marshal([]ReplyToZapierEvent{payload})
    if err != nil {
        z.log.Errorf("error marshaling payload to JSON: %v", err)
        return err
    }

    // Create a new HTTP request with the JSON payload
    req, err := http.NewRequest("POST", webhookUrl, bytes.NewBuffer(jsonData))
    if err != nil {
        z.log.Errorf("error creating HTTP request: %v", err)
        return err
    }

    // Set the Content-Type header to specify JSON data
    req.Header.Set("Content-Type", "application/json")

    // Send the HTTP request
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        z.log.Errorf("error sending HTTP request: %v", err)
        return err
    }
    defer resp.Body.Close()

    // Check the response status code
    if resp.StatusCode != http.StatusOK {
        z.log.Errorf("received non-OK status code: %v", resp.StatusCode)
        return err
    }

    return nil
}
