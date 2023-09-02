package lineEventProcessor

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/slackUtil"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
)

func ProcessFollowEvent(event *linebot.Event,
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
    } else {
        log.Debug("Successfully notified Slack channel of new user follow event")
    }

    // get LINE username
    lineUserProfile, err := line.GetUser(userId)
    if err != nil {
        log.Error("Error getting LINE user profile:", err)
    } else {
        log.Debug("Successfully retrieved LINE user profile:", jsonUtil.AnyToJson(lineUserProfile))
    }

    // if not exists, create new user in DB
    user := model.NewUser(userId, lineUserProfile, event.Timestamp)
    err = userDao.CreateUser(user)
    if err != nil {
        if userAlreadyExistErr, ok := err.(*exception.UserAlreadyExistException); ok {
            log.Info("User already exists. No action taken on Follow event:", userAlreadyExistErr.Error())
            // return 200 OK
        } else {
            log.Error("Error creating user:", err)

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to create user: %s"}`, err),
            }, nil
        }
    }

    log.Debug("Successfully created new user in DB from Follow event:", user)

    log.Info("Successfully handled Follow event for user: ", userId)

    return events.LambdaFunctionURLResponse{Body: `{"message": "Successfully handled Follow event"}`, StatusCode: 200}, nil
}
