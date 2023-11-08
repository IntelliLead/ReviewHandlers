package message

import (
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "time"
)

var TestRealReplyEvent = []*linebot.Event{
    {
        Type:           linebot.EventTypeMessage,
        WebhookEventID: "01H1NCFZSJN1HAPFREM0193Y1Q",
        DeliveryContext: linebot.DeliveryContext{
            IsRedelivery: false,
        },
        Timestamp: time.UnixMilli(1685418671895),
        Source: &linebot.EventSource{
            Type:   linebot.EventSourceTypeUser,
            UserID: "Ucc29292b212e271132cee980c58e94eb",
        },
        Message: &linebot.TextMessage{
            ID:   "480734799091728450",
            Text: "@0|0X 给你们明空",
        },
        ReplyToken: "8174b3e7038f49e6b50fcf17f36b373b",
        Mode:       linebot.EventModeActive,
    },
}
