package lineEventsHandlerTestEvents

import (
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "time"
)

var TestFollowEvent = []*linebot.Event{
    {
        Type:           linebot.EventTypeFollow,
        WebhookEventID: "01H1NCFZSJN1HAPFREM0193Y1Q",
        DeliveryContext: linebot.DeliveryContext{
            IsRedelivery: false,
        },
        // Timestamp: 1685418671895,
        // epoch ms to time.Time
        Timestamp: time.UnixMilli(1685418671895),
        Source: &linebot.EventSource{
            Type:   linebot.EventSourceTypeUser,
            UserID: "Ucc29292b212e271132cee980c58e94eb",
        },
        ReplyToken: "36ffd31138354b2dbe94d1a7759fb9ab",
        Mode:       linebot.EventModeActive,
    },
}
