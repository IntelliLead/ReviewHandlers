package postback

import (
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "time"
)

var TestEditQuickReplyMessage = []*linebot.Event{
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
            Data: "/QuickReply/accounts/106775638291982182570/locations/12251512170589559833/EditQuickReplyMessage",
        },
        ReplyToken: "57146e7d47054573a72ec9958952be46",
        Mode:       linebot.EventModeActive,
    },
}
