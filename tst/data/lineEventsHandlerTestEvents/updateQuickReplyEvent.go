package lineEventsHandlerTestEvents

import (
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "time"
)

var TestUpdateQuickReplyEvent = []*linebot.Event{
    {
        Type:           linebot.EventTypeMessage,
        WebhookEventID: "01H1NCFZSJN1HAPFREM0193Y1Q",
        DeliveryContext: linebot.DeliveryContext{
            IsRedelivery: false,
        },
        Timestamp: time.UnixMilli(1687226076325),
        Source: &linebot.EventSource{
            Type:   linebot.EventSourceTypeUser,
            UserID: "Ucc29292b212e271132cee980c58e94eb",
        },
        Message: &linebot.TextMessage{
            ID:   "460358036163657844",
            Text: "/QuickReply 感謝評價！",
        },
        ReplyToken: "c9ea1fc15d9f498e9955897187869905",
        Mode:       linebot.EventModeActive,
    },
}
