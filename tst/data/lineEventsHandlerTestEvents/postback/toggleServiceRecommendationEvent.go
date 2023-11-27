package postback

import (
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "time"
)

var TestToggleServiceRecommendationEvent = []*linebot.Event{
    {
        Type:           linebot.EventTypePostback,
        WebhookEventID: "01H1NCFZSJN1HAPFREM0193Y1Q",
        DeliveryContext: linebot.DeliveryContext{
            IsRedelivery: false,
        },
        Timestamp: time.UnixMilli(1687226076325),
        Source: &linebot.EventSource{
            Type:   linebot.EventSourceTypeUser,
            UserID: "Ucc29292b212e271132cee980c58e94eb",
        },
        Postback: &linebot.Postback{
            Data: "/AiReply/4496688115335717986/Toggle/ServiceRecommendation",
        },
        ReplyToken: "TST",
        Mode:       linebot.EventModeActive,
    },
}
