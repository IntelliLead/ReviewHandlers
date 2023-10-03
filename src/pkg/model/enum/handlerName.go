package enum

type HandlerName int

const (
    HandlerNameLineEventsHandler HandlerName = iota
    HandlerNameNewReviewEventHandler
    HandlerNameAuthHandler
)

func (s HandlerName) String() string {
    return []string{
        "lineEventsHandler",
        "newReviewEventHandler",
        "authHandler",
    }[s]
}
