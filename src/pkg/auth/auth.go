package auth

import (
    "errors"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "go.uber.org/zap"
)

// ValidateUserAuthOrRequestAuthTst in testing, there is no replyToken, send to user instead.
func ValidateUserAuthOrRequestAuthTst(
    userId string,
    userDao *ddbDao.UserDao,
    businessDao *ddbDao.BusinessDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger,
) (bool, *model.User, *model.Business, error) {
    return ValidateUserAuthOrRequestAuth("TST", userId, userDao, businessDao, line, log)
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
    businessDao *ddbDao.BusinessDao,
    line *lineUtil.Line,
    log *zap.SugaredLogger,
) (bool, *model.User, *model.Business, error) {
    hasUserCompletedOauth, user, business, err := ValidateUserAuth(userId, userDao, businessDao, log)
    if err != nil {
        log.Errorf("Error checking if user %s has completed oauth: %s", userId, err)
        return false, user, business, err
    }

    if hasUserCompletedOauth {
        log.Infof("User %s has completed oauth", userId)
        return true, user, business, nil
    }

    if user == nil {
        log.Info("User with id " + userId + " does not exist. Requesting auth")
    } else if business == nil {
        log.Info("Business does not exist. User '%s' has not been backfilled. Requesting auth", userId)
    } else if !hasUserCompletedOauth {
        log.Info("User %s has associated business %s, but has not completed oauth. Requesting auth", userId, business.BusinessId)
    }

    // when testing in local, there is no replyToken, send to user instead of replying
    if replyToken == "TST" {
        err = line.SendAuthRequest(userId)
    } else {
        err = line.ReplyAuthRequest(replyToken, userId)
    }
    if err != nil {
        log.Errorf("Error replying auth request: %s", err)
        return false, user, business, err
    }
    log.Info("Sent auth request to user ", userId)

    return false, user, business, nil
}

func ValidateUserAuth(
    userId string,
    userDao *ddbDao.UserDao,
    businessDao *ddbDao.BusinessDao,
    logger *zap.SugaredLogger) (bool, *model.User, *model.Business, error) {
    user, err := userDao.GetUser(userId)
    if err != nil {
        logger.Error("Error getting user: ", err)
        return false, nil, nil, err
    }

    if user == nil {
        return false, nil, nil, nil
    }

    // user not yet backfilled
    if user.ActiveBusinessId == nil {
        return false, user, nil, nil
    }

    business, err := businessDao.GetBusiness(*user.ActiveBusinessId)
    if err != nil {
        logger.Errorf("Error getting business with %s for user %s: %s", *user.ActiveBusinessId, userId, err)
        return false, user, nil, err
    }

    if business == nil {
        logger.Errorf("Business with id %s does not exist", *user.ActiveBusinessId)
        return false, user, nil, errors.New("business with id " + *user.ActiveBusinessId + " does not exist")
    }

    if business.Google == nil {
        return false, user, business, nil
    }

    return true, user, business, nil
}
