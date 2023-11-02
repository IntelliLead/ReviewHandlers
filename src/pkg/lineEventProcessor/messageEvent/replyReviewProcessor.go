package messageEvent

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineEventProcessor"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/rid"
    "github.com/aws/aws-lambda-go/events"
    "github.com/line/line-bot-sdk-go/v7/linebot"
    "go.uber.org/zap"
    "strings"
)

// ProcessReviewReplyMessage performs validation of a review reply request and invokes the reply review handler to process the request
func ProcessReviewReplyMessage(
    user model.User,
    event *linebot.Event,
    reviewDao *ddbDao.ReviewDao,
    businessDao *ddbDao.BusinessDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger) (events.LambdaFunctionURLResponse, error) {

    textMessage := event.Message.(*linebot.TextMessage)
    message := textMessage.Text

    // parse reply message
    // --------------------------------
    reply, err := lineEventProcessor.ParseReplyMessage(message)
    if err != nil {
        log.Error("Error parsing reply message:", err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       fmt.Sprintf(`{"error": "Failed to parse reply message: %s"}`, err),
        }, err
    }

    // fetch review from DDB
    // --------------------
    // TODO: [INT-91] remove this after all users have been backfilled
    isUnbackfilledReview := false
    var review model.Review
    var businessId bid.BusinessId
    // check if there is '|', indicating both businessId index and reviewId are provided
    if strings.Contains(reply.UserReviewId.String(), "|") {
        businessIdIndex, rid, err := reply.UserReviewId.Decode()
        if err != nil {
            log.Errorf("Error decoding userReviewId '%s': %v", reply.UserReviewId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to parse userReviewId '%s': %v"}`, reply.UserReviewId, err),
            }, err
        }

        businessId, err := user.GetBusinessIdFromIndex(businessIdIndex)
        if err != nil {
            log.Errorf("Error getting businessId from index %d: %v", businessIdIndex, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to get businessId from index %d: %v"}`, businessIdIndex, err),
            }, err
        }

        reviewPtr, err := reviewDao.GetReview(businessId.String(), rid)
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

        review = *reviewPtr
    } else {
        // try to find user with userID + reviewID
        reviewId, err := rid.NewReviewId(reply.UserReviewId.String())
        if err != nil {
            log.Errorf("Error decoding userReviewId '%s': %v", reply.UserReviewId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to parse userReviewId '%s': %v"}`, reply.UserReviewId, err),
            }, err
        }
        reviewPtr, err := reviewDao.GetReview(user.UserId, reviewId)
        if err != nil {
            log.Errorf("Error getting review %s with userId %s: %s", reply.UserReviewId, user.UserId, jsonUtil.AnyToJson(err))
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to get review: %s"}`, err),
            }, err
        }
        if reviewPtr == nil {
            log.Errorf("Review for reviewId %s with userId %s not found", reply.UserReviewId, user.UserId)

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Review not found"}`),
            }, nil
        }
        isUnbackfilledReview = true
        review = *reviewPtr
    }

    log.Info("Processing reply for review: ", jsonUtil.AnyToJson(review))

    // validate message does not contain LINE emojis
    // --------------------------------
    if HasLineEmoji(textMessage) {
        _, err := line.ReplyUserReviewReplyFailedWithReason(event.ReplyToken, review.ReviewerName,
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

    // Notify review replied on LINE
    // --------------------
    // TODO: [INT-91] remove this after all users have been backfilled
    if isUnbackfilledReview {
        err = line.NotifyReviewReplied([]string{user.UserId}, &event.ReplyToken, &user.UserId, review, reply.Message, user.LineUsername, false, nil)
        if err != nil {
            log.Errorf("Error sending review reply notification to user '%s' for review '%s': %v", user.UserId, review.ReviewId.String(), err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to send review reply notification to user '%s' for review '%s': %v"}`, user.UserId, review.ReviewId.String(), err),
            }, err
        }
    } else {
        business, err := businessDao.GetBusiness(businessId)
        if err != nil {
            log.Errorf("Error getting business '%s': %v", businessId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to get business '%s': %v"}`, businessId, err),
            }, err
        }
        err = line.NotifyReviewReplied(business.UserIds, &event.ReplyToken, &user.UserId, review, reply.Message, user.LineUsername, false, &business.BusinessName)
        if err != nil {
            log.Errorf("Error sending review reply notification to users '%s' of business '%s' for review '%s': %v", business.UserIds, businessId, review.ReviewId.String(), err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to send review reply notification to users '%s' of business '%s' for review '%s': %v"}`, business.UserIds, businessId, review.ReviewId.String(), err),
            }, err
        }
    }

    log.Infof("Successfully handled review reply event to review '%s' for user '%s' with business '%s'", review.ReviewId.String(), user.UserId, businessId)
    return events.LambdaFunctionURLResponse{
        StatusCode: 200,
        Body:       fmt.Sprintf(`{"message": "Successfully handled review reply event for user ID '%s'"}`, user.UserId),
    }, nil

}
