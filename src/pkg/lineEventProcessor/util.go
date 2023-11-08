package lineEventProcessor

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "regexp"
    "strings"
    "unicode"
)

// ParsePostBackData parses the postback data from LINE
// Each argument is separated by "/"
// The only exception is, BusinessID is treated as a single argument
// e.g., "/AiReply/accounts/123/locations/456" -> ["AiReply", "accounts/123/locations/456"]
func ParsePostBackData(data string) ([]string, error) {
    // Regular expression to match the business ID format
    re := regexp.MustCompile(`accounts/\d+/locations/\d+`)

    // Find the business ID in the input data
    businessID := re.FindString(data)

    // If a business ID is found, replace it with a placeholder
    if businessID != "" {
        data = strings.Replace(data, businessID, "{BUSINESS_ID}", 1)
    }

    // Split the data by "/"
    dataSlice := strings.Split(data, "/")

    // Replace the placeholder with the actual business ID
    for i, s := range dataSlice {
        if s == "{BUSINESS_ID}" {
            dataSlice[i] = businessID
        }
    }

    // Shift off the first element, which is empty
    dataSlice = dataSlice[1:]

    if len(dataSlice) == 0 {
        return nil, errors.New("empty data")
    }

    return dataSlice, nil
}

func IsReviewReplyMessage(message string) bool {
    return strings.HasPrefix(message, "@")
}

// CommandMessage format: "/<command> <arg>"
// e.g., "/help xxx"
type CommandMessage struct {
    Command []string
    Arg     string
}

// ParseCommandMessage parses a command message
func ParseCommandMessage(str string) (CommandMessage, error) {
    if !strings.HasPrefix(str, "/") {
        return CommandMessage{}, fmt.Errorf("message is not a command message because it does not begin with '/': %s", str)
    }

    // Find the first whitespace character after '/'
    index := strings.IndexFunc(str[1:], isWhitespace)
    if index == -1 {
        cmd, err := ParsePostBackData(str)
        if err != nil {
            return CommandMessage{}, fmt.Errorf("message '%s' is not a command message: %s", str, err)
        }
        return CommandMessage{
            Command: cmd,
            Arg:     "",
        }, nil
    }

    cmdStr := str[:index+1]
    trimmedArg := strings.TrimSpace(str[index+2:])

    cmd, err := ParsePostBackData(cmdStr)
    if err != nil {
        return CommandMessage{}, fmt.Errorf("message '%s' is not a command message: %s", str, err)
    }

    return CommandMessage{Command: cmd, Arg: trimmedArg}, nil
}

func ParseReplyMessage(str string) (model.Reply, error) {
    if !strings.HasPrefix(str, "@") {
        return model.Reply{}, fmt.Errorf("message is not a reply message: %s", str)
    }

    // Find the first whitespace character after '@'
    index := strings.IndexFunc(str[1:], isWhitespace)
    if index == -1 {
        userReviewId, err := model.ParseUserReviewId(str[1:])
        if err != nil {
            return model.Reply{}, err
        }

        return model.NewReply(userReviewId, "") // Return the remaining text after '@' as UserReviewId
    }

    userReviewId, err := model.ParseUserReviewId(str[1 : index+1])
    if err != nil {
        return model.Reply{}, err
    }

    replyMsg := strings.TrimSpace(str[index+2:])

    return model.NewReply(userReviewId, replyMsg)
}

func isWhitespace(r rune) bool {
    return unicode.IsSpace(r)
}
