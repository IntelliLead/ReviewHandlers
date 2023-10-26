package lineEventProcessor

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/auth"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/metric"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/metric/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/slackUtil"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
)

func ProcessFollowEvent(event *linebot.Event,
    businessDao *ddbDao.BusinessDao,
    userDao *ddbDao.UserDao,
    slack *slackUtil.Slack,
    line *lineUtil.Line,
    log *zap.SugaredLogger) (events.LambdaFunctionURLResponse, error) {

    if lineUtil.IsEventFromUser(event) == false {
        log.Info("Message is not from user. Ignoring event")
        return events.LambdaFunctionURLResponse{Body: `{"message": "OK"}`, StatusCode: 200}, nil
    }

    userId := event.Source.UserID

    // notify Slack channel
    err := slack.SendNewUserFollowedMessage(userId, event.Timestamp)
    if err != nil {
        log.Error("Error sending Slack message:", err)
        metric.EmitLambdaMetric(enum2.Metric5xxError, enum.HandlerNameLineEventsHandler, 1.0)

    }

    log.Info("Successfully notified Slack channel of new user follow event")

    var hasUserAuthed bool
    hasUserAuthed, _, _, err = auth.ValidateUserAuthOrRequestAuth(event.ReplyToken, userId, userDao, businessDao, line, log)
    if err != nil {
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to validate user auth: %s"}`, err),
        }, err
    }
    if !hasUserAuthed {
        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "User has not authenticated. Requested authentication."}`,
        }, nil
    }

    log.Info("Successfully handled Follow event for user: ", userId)

    return events.LambdaFunctionURLResponse{Body: `{"message": "Successfully handled Follow event"}`, StatusCode: 200}, nil
}
