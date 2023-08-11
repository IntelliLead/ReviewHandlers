package messageEvent

import (
    "github.com/line/line-bot-sdk-go/v7/linebot"
)

func HasLineEmoji(textMessage *linebot.TextMessage) bool {
    emojis := textMessage.Emojis
    return len(emojis) > 0
}
