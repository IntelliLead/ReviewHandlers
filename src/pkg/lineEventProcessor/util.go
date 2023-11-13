package lineEventProcessor

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "strings"
    "unicode"
)

// ParsePostBackData parses the postback data from LINE
// Each argument is separated by "/"
// e.g., "/AiReply/123456" -> ["AiReply", "123456"]
func ParsePostBackData(data string) ([]string, error) {
    dataSlice := strings.Split(data, "/")

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
