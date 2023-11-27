package messageEvent

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineEventProcessor"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/metric"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/metric/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
)

// ProcessReviewReplyMessage performs validation of a review reply request and invokes the reply review handler to process the request
func ProcessReviewReplyMessage(
    user model.User,
    event *linebot.Event,
    reviewDao *ddbDao.ReviewDao,
    businessDao *ddbDao.BusinessDao,
    userDao *ddbDao.UserDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) (events.LambdaFunctionURLResponse, error) {

    textMessage := event.Message.(*linebot.TextMessage)
    message := textMessage.Text

    // --------------------------------
    // parse reply message
    // --------------------------------
    reply, err := lineEventProcessor.ParseReplyMessage(message)
    if err != nil {
        log.Error("Error parsing reply message:", err)

        _, notifyUserErr := line.ReplyUserReplyFailedWithReason(event.ReplyToken, "", "格式有錯。請保留 ‘@’ 符號後的編號，並在空格後面輸入回覆內容。")
        if notifyUserErr != nil {
            log.Errorf("Error notifying reply failure to user '%s': %v", user.UserId, notifyUserErr)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to notify reply failure for user '%s' : %v. Reply Failure reason: %v"}`, user.UserId, notifyUserErr, err),
            }, notifyUserErr

        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 400,
            Body:       fmt.Sprintf(`{"error": "Failed to parse reply message: %s"}`, err),
        }, err
    }

    var businessIdIndex int
    if reply.UserReviewId.BusinessIdIndex == nil {
        log.Warn("Error parsing reply message: businessIdIndex is nil. Replying to review without businessIdIndex is not supported")
        // TODO: return error instead once all new reviews should have businessIdIndex - 2 weeks after Nov 7, 2023
        log.Warn("Attempting to continue assuming index is 0")
        businessIdIndex = 0
        metric.EmitLambdaMetric(enum.Metric4xxError, enum2.HandlerNameLineEventsHandler, 1)
    } else {
        businessIdIndex = *reply.UserReviewId.BusinessIdIndex
    }

    businessId, err := user.GetBusinessIdFromIndex(businessIdIndex)
    if err != nil {
        log.Errorf("Error getting businessId from index %d: %v", *reply.UserReviewId.BusinessIdIndex, err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to get businessId from index %d: %v"}`, *reply.UserReviewId.BusinessIdIndex, err),
        }, err
    }
    reviewId := reply.UserReviewId.ReviewId

    // --------------------------------
    // fetch review from DDB
    // --------------------
    reviewPtr, err := reviewDao.GetReview(businessId.String(), reviewId)
    if err != nil {
        log.Errorf("Error getting review %s with businessId %s: %s", reply.UserReviewId, businessId, jsonUtil.AnyToJson(err))
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to get review: %s"}`, err),
        }, err
    }
    if reviewPtr == nil {
        log.Errorf("Review for reviewId %s with businessId %s not found", reply.UserReviewId, businessId)

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Review not found"}`),
        }, nil
    }
    review := *reviewPtr

    log.Info("Processing reply for review: ", jsonUtil.AnyToJson(review))

    // --------------------------------
    // validate message does not contain LINE emojis
    // --------------------------------
    if HasLineEmoji(textMessage) {
        _, err := line.ReplyUserReplyFailedWithReason(event.ReplyToken, review.ReviewerName,
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

    // --------------------------------
    // process reply message
    // --------------------------------
    err = lineEventProcessor.ReplyReview(user.UserId, reply.Message, review, reviewDao, log)
    if err != nil {
        log.Errorf("Error handling replying '%s' to review '%s' for user '%s' business '%s': %v", jsonUtil.AnyToJson(reply.Message), review.ReviewId.String(), user.UserId, businessId, err)

        _, notifyUserErr := line.ReplyUserReplyFailed(event.ReplyToken, review.ReviewerName, false)
        if notifyUserErr != nil {
            log.Errorf("Error notifying user '%s' reply failed for review '%s': %v", user.UserId, review.ReviewId.String(), notifyUserErr)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to notify reply failure for user '%s' : %v. Reply Failure reason: %v"}`, user.UserId, notifyUserErr, err),
            }, notifyUserErr
        }

        log.Infof("Successfully notified user '%s' business '%s' reply to review '%s' failed: %v",
            user.UserId, businessId, review.ReviewId.String(), err)

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Reply failed: %s"}`, err),
        }, err
    }

    // --------------------------------
    // Notify review replied on LINE
    // --------------------
    businessPtr, err := businessDao.GetBusiness(businessId)
    if err != nil {
        log.Errorf("Error getting business '%s': %v", businessId, err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to get business '%s': %v"}`, businessId, err),
        }, err
    }
    if businessPtr == nil {
        log.Errorf("Business '%s' not found", businessId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Business '%s' not found"}`, businessId),
        }, nil
    }
    business := *businessPtr
    err = line.NotifyReviewReplied(event.ReplyToken, review, reply.Message, business, user, userDao)
    if err != nil {
        log.Errorf("Error sending review reply notification to users '%s' of business '%s' for review '%s': %v", business.UserIds, businessId, review.ReviewId.String(), err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to send review reply notification to users '%s' of business '%s' for review '%s': %v"}`, business.UserIds, businessId, review.ReviewId.String(), err),
        }, err
    }

    log.Infof("Successfully handled review reply event to review '%s' for user '%s' with business '%s'", review.ReviewId.String(), user.UserId, businessId)
    return events.LambdaFunctionURLResponse{
        StatusCode: 200,
        Body:       fmt.Sprintf(`{"message": "Successfully handled review reply event for user ID '%s'"}`, user.UserId),
    }, nil
}
