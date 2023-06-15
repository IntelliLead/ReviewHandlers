package slackUtil

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/secret"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/slack-go/slack"
    "go.uber.org/zap"
    "time"
)

/*
Slack CLI is unfortunately a paid feature: https://api.slack.com/automation/quickstart
So we have to use the web API instead: https://api.slack.com/web
*/

type Slack struct {
    client *slack.Client
    log    *zap.SugaredLogger
    stage  enum.Stage
}

func NewSlack(logger *zap.SugaredLogger) *Slack {
    client := slack.New(secret.GetSecrets().SlackToken)

    // DEBUG: Test authentication
    // _, err := client.AuthTest()
    // if err != nil {
    //     logger.Error("Unable to authenticate with Slack: ", err)
    //     return nil
    // }

    return &Slack{
        client: client,
        log:    logger,
    }
}

var channelId = secret.GetSecrets().NewUserSlackBotChannelId

func (s *Slack) SendNewUserFollowedMessage(userId string, timestamp time.Time) error {
    readableTimestamp, err := util.UtcToReadableTwTimestamp(timestamp)
    if err != nil {
        s.log.Error("Unable to convert timestamp to readable format in SendNewUserFollowedMessage: ", err)
        return err
    }

    msg1 := ""
    if s.stage != enum.StageProd {
        msg1 += "*[" + s.stage.String() + "]* "
    }
    msg1 += "New user followed IntelliLead App LINE Official Account at " + readableTimestamp + ". User ID: "
    respChannel, respTimestamp, err := s.client.PostMessage(
        channelId,
        slack.MsgOptionText(msg1, false),
    )
    if err != nil {
        s.log.Error("Unable to send message 1 to slack in SendNewUserFollowedMessage: ", err)
        return err
    }

    s.log.Debugf("Message 1 successfully sent to slack channel %s at %s", respChannel, respTimestamp)

    blocks := []slack.Block{
        slack.NewSectionBlock(slack.NewTextBlockObject(slack.PlainTextType, userId, false, false), nil, nil),
        slack.NewDividerBlock(),
    }
    respChannel, respTimestamp, err = s.client.PostMessage(
        channelId,
        slack.MsgOptionBlocks(blocks...),
    )
    if err != nil {
        s.log.Error("Unable to send message 2 to slack in SendNewUserFollowedMessage: ", err)
        return err
    }

    s.log.Debugf("Message 2 successfully sent to slack channel %s at %s", respChannel, respTimestamp)

    return nil
}
