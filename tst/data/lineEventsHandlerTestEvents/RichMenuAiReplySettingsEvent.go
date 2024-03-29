package lineEventsHandlerTestEvents

import (
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "time"
)

var TestRichMenuAiReplySettingsEvent = []*linebot.Event{
    {
        Type:           linebot.EventTypePostback,
        WebhookEventID: "01H39TXHY0CR820VK6527WW2QN",
        DeliveryContext: linebot.DeliveryContext{
            IsRedelivery: false,
        },
        Timestamp: time.UnixMilli(1687178626854),
        Source: &linebot.EventSource{
            Type:   linebot.EventSourceTypeUser,
            UserID: "Ucc29292b212e271132cee980c58e94eb",
        },
        Postback: &linebot.Postback{
            Data: "/RichMenu/AiReplySettings",
        },
        ReplyToken: "TST",
        Mode:       linebot.EventModeActive,
    },
}
