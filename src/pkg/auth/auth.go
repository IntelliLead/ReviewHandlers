package auth

import (
    "errors"
    "fmt"
    jsonUtil2 "github.com/IntelliLead/CoreCommonUtil/jsonUtil"
    "github.com/IntelliLead/CoreCommonUtil/metric"
    "github.com/IntelliLead/CoreCommonUtil/metric/enum"
    "github.com/IntelliLead/CoreCommonUtil/stringUtil"
    "github.com/IntelliLead/CoreDataAccess/ddbDao"
    "github.com/IntelliLead/CoreDataAccess/ddbDao/dbModel"
    enum3 "github.com/IntelliLead/CoreDataAccess/ddbDao/enum"
    "github.com/IntelliLead/CoreDataAccess/exception"
    "github.com/IntelliLead/CoreDataAccess/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "go.uber.org/zap"
)

// ValidateUserAuthOrRequestAuthTst in testing, there is no replyToken, send to user instead.
func ValidateUserAuthOrRequestAuthTst(
    userId string,
    userDao *ddbDao.UserDao,
    line *lineUtil.Line,
    handlerName enum2.HandlerName,
    log *zap.SugaredLogger,
) (bool, *model.User, error) {
    return ValidateUserAuthOrRequestAuth("TST", userId, userDao, line, handlerName, log)
}

// ValidateUserAuthOrRequestAuth checks if the user has completed oauth.
// If not, it sends an auth request to the user. Invoker would terminate its execution after this call.
// Returns: hasUserAuthed, user, business, error
// if hasUserAuthed is true, user and business will not be nil
// If user has not completed oauth, hasUserAuthed will be false, and business may be nil
// If user is not found, hasUserAuthed will be false, and user will also be nil
func ValidateUserAuthOrRequestAuth(
    replyToken string,
    userId string,
    userDao *ddbDao.UserDao,
    line *lineUtil.Line,
    handlerName enum2.HandlerName,
    log *zap.SugaredLogger,
) (bool, *model.User, error) {
    hasUserCompletedOauth, user, err := ValidateUserAuth(userId, userDao, line, handlerName, log)
    if err != nil {
        var userDoesNotExistException *exception.UserDoesNotExistException
        if errors.As(err, &userDoesNotExistException) {
            err = requestAuth(replyToken, userId, line, log)
            if err != nil {
                metric.EmitLambdaMetric(enum.Metric5xxError, handlerName.String(), 1)
            }
            return false, nil, nil
        }

        log.Errorf("Error checking if user %s has completed OAUTH: %s", userId, err)
        return false, nil, err
    }

    if !hasUserCompletedOauth {
        log.Info("User ", userId, " has not completed OAUTH. Sending auth request.")
        err = requestAuth(replyToken, userId, line, log)
        if err != nil {
            metric.EmitLambdaMetric(enum.Metric5xxError, handlerName.String(), 1)
        }
    } else {
        log.Info("User ", userId, " has completed OAUTH.")
    }

    return hasUserCompletedOauth, &user, nil
}

func requestAuth(
    replyToken string,
    userId string,
    line *lineUtil.Line,
    log *zap.SugaredLogger,
) error {
    // when testing in local, there is no replyToken, send to user instead of replying
    var err error
    if replyToken == "TST" {
        err = line.SendAuthRequest(userId)
    } else {
        err = line.ReplyAuthRequest(replyToken, userId)
    }
    if err != nil {
        log.Errorf("Error replying auth request: %s", err)
        return err
    }
    log.Info("Sent auth request to user ", userId)

    return nil
}

// TODO: [INT-91] remove this check after LINE user info backfilling is done
func backfillLineUserInfo(user *model.User, userDao *ddbDao.UserDao, line *lineUtil.Line, handlerName enum2.HandlerName, log *zap.SugaredLogger) {
    if stringUtil.IsEmptyString(user.LineUsername) || stringUtil.IsEmptyString(user.LineProfilePictureUrl) || stringUtil.IsEmptyString(user.Language) {
        lineGetUserResp, err := line.GetUser(user.UserId)
        if err != nil {
            log.Errorf("Error getting user info from LINE: %s", err)
            metric.EmitLambdaMetric(enum.Metric5xxError, handlerName.String(), 1)
            return // do not backfill
        }

        lineUserNameAction, err := dbModel.NewAttributeAction(enum3.ActionUpdate, "lineUsername", lineGetUserResp.DisplayName)
        if err != nil {
            log.Errorf("Error creating attribute action: %s", err)
            metric.EmitLambdaMetric(enum.Metric5xxError, handlerName.String(), 1)
            return // do not backfill
        }
        lineProfilePictureUrlAction, err := dbModel.NewAttributeAction(enum3.ActionUpdate, "lineProfilePictureUrl", lineGetUserResp.PictureURL)
        if err != nil {
            log.Errorf("Error creating attribute action: %s", err)
            metric.EmitLambdaMetric(enum.Metric5xxError, handlerName.String(), 1)
            return // do not backfill
        }
        languageAction, err := dbModel.NewAttributeAction(enum3.ActionUpdate, "language", lineGetUserResp.Language)
        if err != nil {
            log.Errorf("Error creating attribute action: %s", err)
            metric.EmitLambdaMetric(enum.Metric5xxError, handlerName.String(), 1)
            return // do not backfill
        }

        updatedUser, err := userDao.UpdateAttributes(user.UserId, []dbModel.AttributeAction{lineUserNameAction, lineProfilePictureUrlAction, languageAction})
        if err != nil {
            log.Errorf("Error updating user info: %s", err)
            metric.EmitLambdaMetric(enum.Metric5xxError, handlerName.String(), 1)
            return
        }

        *user = updatedUser
        log.Info("Successfully backfilled user's line info: ", jsonUtil2.AnyToJson(updatedUser))
    }
}

// ValidateUserAuth checks if the user has completed oauth.
// Returns: hasUserAuthed, user, business, error
// if hasUserAuthed is true, user and business will not be nil
// If user has not completed oauth, hasUserAuthed will be false, and business may be nil
// If user is not found, hasUserAuthed will be false, and user will also be nil
func ValidateUserAuth(
    userId string,
    userDao *ddbDao.UserDao,
    line *lineUtil.Line,
    handlerName enum2.HandlerName,
    logger *zap.SugaredLogger) (bool, model.User, error) {
    userPtr, err := userDao.GetUser(userId)
    if err != nil {
        logger.Error("Error getting user: ", err)
        return false, model.User{}, err
    }

    if userPtr == nil {
        return false, model.User{}, exception.NewUserDoesNotExistException(fmt.Sprintf("User with id %s does not exist", userId), nil)
    }

    backfillLineUserInfo(userPtr, userDao, line, handlerName, logger)

    user := *userPtr

    // either condition works, checking both for robustness
    if len(user.BusinessIds) == 0 || stringUtil.IsEmptyString(user.Google.RefreshToken) {
        return false, user, nil
    }

    return true, user, nil
}
