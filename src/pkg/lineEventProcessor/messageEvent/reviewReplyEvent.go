package messageEvent

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineEventProcessor"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
)

func ProcessReviewReplyMessage(
    business model.Business,
    user model.User,
    event *linebot.Event,
    reviewDao *ddbDao.ReviewDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) (events.LambdaFunctionURLResponse, error) {

    textMessage := event.Message.(*linebot.TextMessage)
    message := textMessage.Text

    // parse reply message
    // --------------------------------
    reply, err := lineUtil.ParseReplyMessage(message)
    if err != nil {
        log.Error("Error parsing reply message:", err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to parse reply message: %s"}`, err),
        }, err
    }

    // fetch review from DDB
    // --------------------
    review, err := reviewDao.GetReview(business.BusinessId, reply.ReviewId)
    if err != nil {
        log.Errorf("Error getting review %s with businessId %s: %s", reply.ReviewId, business.BusinessId, jsonUtil.AnyToJson(err))
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to get review: %s"}`, err),
        }, err
    }
    if review == nil {
        log.Errorf("Review for reviewId %s with businessId %s not found", reply.ReviewId, business.BusinessId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 404,
            Body:       fmt.Sprintf(`{"error": "Review not found"}`),
        }, nil
    }

    log.Debug("Got Review: ", jsonUtil.AnyToJson(review))

    // validate message does not contain LINE emojis
    // --------------------------------
    if HasLineEmoji(textMessage) {
        _, err := line.ReplyUserReviewReplyProcessedWithReason(event.ReplyToken, false, review.ReviewerName,
            lineUtil.CannotUseLineEmojiMessage)
        if err != nil {
            log.Errorf("Error notifying reply failure for user '%s' for review '%s': %v",
                user.UserId, jsonUtil.AnyToJson(review), err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to notify reply failure for user '%s' : %v"}`, user.UserId, err),
            }, err
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       `{"message": "Notified LINE Emoji not yet supported"}`,
        }, nil
    }

    lambdaReturn, err := lineEventProcessor.ReplyReview(business.BusinessId, user.UserId, &event.ReplyToken, reply.Message, *review, reviewDao, line, log, false)
    if err != nil {
        return lambdaReturn, err
    }

    log.Infof("Successfully handled review reply event for user ID '%s', reply '%s' for business ID '%s' review ID '%s'",
        user.UserId, jsonUtil.AnyToJson(reply), business.BusinessId, review.ReviewId.String())

    return lambdaReturn, nil
}
