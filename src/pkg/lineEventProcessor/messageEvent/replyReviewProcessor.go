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

// ProcessReviewReplyMessage performs validation of a review reply request and invokes the reply review handler to process the request
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
    reviewPtr, err := reviewDao.GetReview(business.BusinessId, reply.ReviewId)
    if err != nil {
        log.Errorf("Error getting review %s with businessId %s: %s", reply.ReviewId, business.BusinessId, jsonUtil.AnyToJson(err))
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to get review: %s"}`, err),
        }, err
    }
    if reviewPtr == nil {
        log.Errorf("Review for reviewId %s with businessId %s not found", reply.ReviewId, business.BusinessId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 404,
            Body:       fmt.Sprintf(`{"error": "Review not found"}`),
        }, nil
    }

    review := *reviewPtr

    log.Info("Processing reply for review: ", jsonUtil.AnyToJson(review))

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

    err = lineEventProcessor.ReplyReview(business.BusinessId, user.UserId, reply.Message, review, reviewDao, log)
    if err != nil {
        log.Errorf("Error handling replying '%s' to review '%s' for user '%s' of business '%s': %v", jsonUtil.AnyToJson(reply.Message), review.ReviewId.String(), user.UserId, business.BusinessId, err)

        _, notifyUserErr := line.ReplyUserReplyFailed(event.ReplyToken, review.ReviewerName, false)
        if notifyUserErr != nil {
            log.Errorf("Error notifying user '%s' reply failed for review '%s': %v", user.UserId, review.ReviewId.String(), notifyUserErr)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to notify reply failure for user '%s' : %v. Reply Failure reason: %v"}`, user.UserId, notifyUserErr, err),
            }, notifyUserErr
        }

        log.Infof("Successfully notified user '%s' of business '%s' reply to review '%s' failed: %v",
            user.UserId, business.BusinessId, review.ReviewId.String(), err)

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Reply failed: %s"}`, err),
        }, err
    }

    // send LINE message
    // --------------------
    err = line.NotifyReviewReplied(business.UserIds, &event.ReplyToken, &user.UserId, review, reply.Message, user.LineUsername, false)
    if err != nil {
        log.Errorf("Error sending review reply notification to all users of business '%s': %v", business.BusinessId, err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to send review reply notification to all users of business '%s': %v"}`, business.BusinessId, err),
        }, err
    }

    log.Infof("Successfully handled review reply event to review '%s' for user '%s' of business '%s'", review.ReviewId.String(), user.UserId, business.BusinessId)
    return events.LambdaFunctionURLResponse{
        StatusCode: 200,
        Body:       fmt.Sprintf(`{"message": "Successfully handled review reply event for user ID '%s'"}`, user.UserId),
    }, nil

}
