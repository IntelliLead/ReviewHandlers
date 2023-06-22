package lineEventProcessor

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/aiUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
    "strings"
)

func ProcessPostbackEvent(event *linebot.Event,
    userId string,
    userDao *ddbDao.UserDao,
    reviewDao *ddbDao.ReviewDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) (events.LambdaFunctionURLResponse, error) {

    // parse data - expect path format: e.g., /RichMenu/QuickReplySettings
    dataSlice := strings.Split(event.Postback.Data, "/")
    if len(dataSlice) < 2 {
        return returnUnhandledPostback(log, *event), nil
    }
    // shift off the first element, which is empty
    dataSlice = dataSlice[1:]

    switch dataSlice[0] {
    case "NewReview", "AiReply":
        // /[NewReview|AiReply]/GenerateAiReply/{REVIEW_ID}
        if len(dataSlice) < 3 || dataSlice[1] != "GenerateAiReply" || util.IsEmptyString(dataSlice[2]) {
            return returnUnhandledPostback(log, *event), nil
        }

        // Notify user that AI is generating reply
        _, err := line.NotifyUserAiReplyGenerationInProgress(event.ReplyToken)
        if err != nil {
            log.Errorf("Error notifying user '%s' that AI is generating reply. Porceeding: %v", userId, err)
        }

        // get review
        reviewIdArg := dataSlice[2]
        review, err := reviewDao.GetReview(userId, _type.NewReviewId(reviewIdArg))
        if err != nil {
            log.Errorf("Error getting review during handling %s: %v", event.Postback.Data, err)

            _, err := line.NotifyUserAiReplyGenerationFailed(userId)
            if err != nil {
                log.Errorf("Error notifying user '%s' that AI reply generation failed: %v", userId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error notifying user that AI reply generation failed: %s"}`, err),
                }, err
            }

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error getting review during handling %s: %s"}`, event.Postback.Data, err),
            }, err
        }

        // invoke gpt4
        aiReply, err := aiUtil.NewAi(log).GenerateReply(review.Review)
        if err != nil {
            log.Errorf("Error generating AI reply: %v", err)

            _, err := line.NotifyUserAiReplyGenerationFailed(userId)
            if err != nil {
                log.Errorf("Error notifying user '%s' that AI reply generation failed: %v", userId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error notifying user that AI reply generation failed: %s"}`, err),
                }, err
            }

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error generating AI reply: %s"}`, err),
            }, err
        }

        // create AI generated result card
        err = line.SendAiGeneratedReply(aiReply, review)
        if err != nil {
            log.Errorf("Error sending AI generated reply to user '%s': %v", userId, err)

            _, err := line.NotifyUserAiReplyGenerationFailed(userId)
            if err != nil {
                log.Errorf("Error notifying user '%s' that AI reply generation failed: %v", userId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error notifying user that AI reply generation failed: %s"}`, err),
                }, err
            }

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Error sending AI generated reply: %s"}`, err),
            }, err
        }

    case "RichMenu":
        if len(dataSlice) < 2 {
            return returnUnhandledPostback(log, *event), nil
        }

        switch dataSlice[1] {
        case "QuickReplySettings":
            // get user
            user, err := userDao.GetUser(userId)
            if err != nil {
                log.Error("Error getting user during handling /RichMenu/QuickReplySettings: ", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error getting user: %s"}`, err),
                }, err
            }

            err = line.ShowQuickReplySettings(event.ReplyToken, user, false)
            if err != nil {
                log.Errorf("Error sending quick reply settings to user '%s': %v", user.UserId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error sending quick reply settings: %s"}`, err),
                }, err
            }

        case "Help":
            _, err := line.ReplyHelpMessage(event.ReplyToken)
            if err != nil {
                log.Errorf("Error replying help message to user '%s': %v", userId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error replying help message: %s"}`, err),
                }, err
            }

        case "More":
            _, err := line.ReplyMoreMessage(event.ReplyToken)
            if err != nil {
                log.Errorf("Error replying More message to user '%s': %v", userId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error replying more message: %s"}`, err),
                }, err
            }

        default:
            log.Error("Unknown RichMenu postback data: ", dataSlice[1])
            return events.LambdaFunctionURLResponse{
                StatusCode: 400,
                Body:       fmt.Sprintf(`{"message": "Unknown RichMenu postback data: %s"}`, dataSlice[1]),
            }, nil
        }

    case "QuickReply":
        if len(dataSlice) < 2 {
            return returnUnhandledPostback(log, *event), nil
        }

        if dataSlice[1] == "DeleteQuickReplyMessage" {
            updatedUser, err := userDao.DeleteQuickReplyMessage(userId)
            if err != nil {
                log.Errorf("Error deleting quick reply message for user '%s': %v", userId, err)

                _, err := line.NotifyUserUpdateQuickReplyMessageFailed(event.ReplyToken)
                if err != nil {
                    return events.LambdaFunctionURLResponse{
                        StatusCode: 500,
                        Body:       fmt.Sprintf(`{"error": "Failed to notify user of delete quick reply message failed: %s"}`, err),
                    }, err
                }
                log.Error("Successfully notified user of update quick reply message failed")

                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Error deleting quick reply message: %s"}`, err),
                }, err
            }

            err = line.ShowQuickReplySettings(event.ReplyToken, updatedUser, true)
            if err != nil {
                log.Errorf("Error showing quick reply settings for user '%s': %v", userId, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       fmt.Sprintf(`{"error": "Failed to show quick reply settings: %s"}`, err),
                }, err
            }

        }
    default:
        log.Warn("Unknown QuickReply postback data: ", dataSlice)
        return events.LambdaFunctionURLResponse{
            StatusCode: 400,
            Body:       "{\"message\": \"Unknown QuickReply postback data\"}",
        }, nil
    }

    log.Infof("Successfully handled Postback event from user '%s': %s", userId, jsonUtil.AnyToJson(event.Postback.Data))

    return events.LambdaFunctionURLResponse{Body: `{"message": "Successfully handled Postback event"}`, StatusCode: 200}, nil
}

func returnUnhandledPostback(log *zap.SugaredLogger, event linebot.Event) events.LambdaFunctionURLResponse {
    log.Error("Postback event data is not in expected format. No action taken: ", event.Postback.Data)
    return events.LambdaFunctionURLResponse{
        StatusCode: 200,
        Body:       `{"message": "Postback event data is not in expected format. No action taken."}`,
    }
}
