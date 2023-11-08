package slackUtil

import (
    "github.com/IntelliLead/ReviewHandlers/src/pkg/awsUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
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
    client    *slack.Client
    log       *zap.SugaredLogger
    stage     enum.Stage
    channelId string
}

func NewSlack(logger *zap.SugaredLogger, stage enum.Stage) *Slack {
    secrets := awsUtil.NewAws(logger).GetSecrets()
    client := slack.New(secrets.SlackToken)

    // DEBUG: Test authentication
    // _, err := client.AuthTest()
    // if err != nil {
    //     logger.Error("Unable to authenticate with Slack: ", err)
    //     return nil
    // }

    return &Slack{
        client:    client,
        log:       logger,
        stage:     stage,
        channelId: secrets.NewUserSlackBotChannelId,
    }
}

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
        s.channelId,
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
        s.channelId,
        slack.MsgOptionBlocks(blocks...),
    )
    if err != nil {
        s.log.Error("Unable to send message 2 to slack in SendNewUserFollowedMessage: ", err)
        return err
    }

    s.log.Debugf("Message 2 successfully sent to slack channel %s at %s", respChannel, respTimestamp)

    return nil
}

func (s *Slack) SendNewUserAssociatedWithBusinessesMessage(userId string, businessIds []bid.BusinessId) error {
    readableTimestamp, err := util.UtcToReadableTwTimestamp(time.Now())
    if err != nil {
        s.log.Error("Unable to convert timestamp to readable format in SendNewUserFollowedMessage: ", err)
        return err
    }

    msg1 := ""
    if s.stage != enum.StageProd {
        msg1 += "*[" + s.stage.String() + "]* "
    }
    msg1 += "New user associated with businesses at " + readableTimestamp + ". User ID: "

    // one businessId per line
    businessIdsStr := ""
    for _, businessId := range businessIds {
        businessIdsStr += "\n" + businessId.String()
    }
    businessIdsStr = businessIdsStr[1:]

    blocks := []slack.Block{
        slack.NewSectionBlock(slack.NewTextBlockObject(slack.PlainTextType, msg1, false, false), nil, nil),
        slack.NewSectionBlock(slack.NewTextBlockObject(slack.PlainTextType, userId, false, false), nil, nil),
        slack.NewSectionBlock(slack.NewTextBlockObject(slack.PlainTextType, "Business ID(s)", false, false), nil, nil),
        slack.NewSectionBlock(slack.NewTextBlockObject(slack.PlainTextType, businessIdsStr, false, false), nil, nil),
        slack.NewDividerBlock(),
    }

    respChannel, respTimestamp, err := s.client.PostMessage(
        s.channelId,
        slack.MsgOptionBlocks(blocks...),
    )
    if err != nil {
        s.log.Error("Unable to send message to slack in SendNewUserAssociatedWithBusinessesMessage: ", err)
        return err
    }

    s.log.Debugf("Message successfully sent to slack channel %s at %s", respChannel, respTimestamp)

    return nil
}
