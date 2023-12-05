package main

import (
    "context"
    "errors"
    enum3 "github.com/IntelliLead/CoreCommonUtil/enum"
    "github.com/IntelliLead/CoreCommonUtil/jsonUtil"
    "github.com/IntelliLead/CoreCommonUtil/logger"
    "github.com/IntelliLead/CoreCommonUtil/metric"
    enum4 "github.com/IntelliLead/CoreCommonUtil/metric/enum"
    "github.com/IntelliLead/CoreCommonUtil/middleware"
    "github.com/IntelliLead/CoreCommonUtil/stringUtil"
    "github.com/IntelliLead/CoreDataAccess/ddbDao"
    "github.com/IntelliLead/CoreDataAccess/ddbDao/dbModel"
    "github.com/IntelliLead/CoreDataAccess/ddbDao/enum"
    "github.com/IntelliLead/CoreDataAccess/exception"
    model2 "github.com/IntelliLead/CoreDataAccess/model"
    "github.com/IntelliLead/CoreDataAccess/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/googleUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/slackUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "golang.org/x/oauth2"
    "google.golang.org/api/mybusinessaccountmanagement/v1"
    "os"
    "strings"
)

func main() {
    lambda.Start(middleware.MetricMiddleware(enum2.HandlerNameAuthHandler.String(), handleRequest))
}

var (
    log = logger.NewLogger()
)

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    stageStr := os.Getenv(util.StageEnvKey)
    stage := enum3.ToStage(stageStr) // panic if invalid stage
    log.Infof("Received request in %s: %s", stage, jsonUtil.AnyToJson(request))

    // ----
    // 1. Validation
    // ----
    // gracefully ignore favicon request
    if request.RequestContext.HTTP.Method == "GET" && request.RequestContext.HTTP.Path == "/favicon.ico" {
        log.Infof("Received favicon request. Ignoring.")

        return events.LambdaFunctionURLResponse{
            StatusCode: 200,
            Body:       "no favicon",
        }, nil
    }

    // check for error from Google OAUTH response
    errorQueryParam := request.QueryStringParameters["error"]
    if errorQueryParam != "" {
        log.Errorf("Error from Google OAUTH response: %s", errorQueryParam)
        return events.LambdaFunctionURLResponse{
            StatusCode: 400,
            Body:       `{"error": "Error from Google OAUTH response"}`,
        }, nil
    }

    // parse the code parameter
    code := request.QueryStringParameters["code"]
    if code == "" {
        log.Errorf("Missing code parameter from Google OAUTH response")
        return events.LambdaFunctionURLResponse{
            StatusCode: 400,
            Body:       `{"error": "Missing code parameter from Google OAUTH response"}`,
        }, nil
    }

    log.Debugf("Received authorization code from Google OAUTH response: %s", code)

    // parse the state parameter
    userId := request.QueryStringParameters["state"]
    if userId == "" {
        log.Errorf("Missing state parameter from Google OAUTH response containing userId.")
        return events.LambdaFunctionURLResponse{
            StatusCode: 400,
            Body:       `{"error": "Missing state parameter from Google OAUTH response"}`,
        }, nil
    }

    log.Info("Received OAUTH request from user: ", userId)

    // ----
    // 2. Initialize resources
    // ----
    cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-northeast-1"))
    if err != nil {
        log.Error("Error loading AWS config: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error loading AWS config"}`, StatusCode: 500}, nil
    }
    businessDao := ddbDao.NewBusinessDao(dynamodb.NewFromConfig(cfg), log)
    userDao := ddbDao.NewUserDao(dynamodb.NewFromConfig(cfg), log)
    line := lineUtil.NewLine(log)

    google, err := googleUtil.NewGoogleWithAuthCode(log, code)
    if err != nil {
        err := line.SendMessage(userId, "驗證失敗。請稍後再試。很抱歉問您造成不便！")
        if err != nil {
            log.Errorf("Error sending LINE message to '%s': %s", userId, err)
            metric.EmitLambdaMetric(enum4.Metric5xxError, enum2.HandlerNameAuthHandler.String(), 1)
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error creating Google OAUTH client"}`,
        }, err
    }

    userPtr, err := userDao.GetUser(userId)
    if err != nil {
        log.Error("Error checking if user exists: ", err)

        err := line.SendMessage(userId, "驗證失敗。請稍後再試。很抱歉問您造成不便！")
        if err != nil {
            log.Errorf("Error sending LINE message to '%s': %s", userId, err)
            metric.EmitLambdaMetric(enum4.Metric5xxError, enum2.HandlerNameAuthHandler.String(), 1)
        }

        return events.LambdaFunctionURLResponse{Body: `{"message": "Error checking if user exists"}`, StatusCode: 500}, err
    }

    // ----
    // 2. update businesses and user
    // ----

    /*
       scenarios:
        1. Neither user nor business exist: create new user and associated business (primary user first time auth)
        2. user exists, but business does not exist: create new business, associate with user, and create/update Google metadata for user (backfill - primary user)
        2. Business exists, but user does not exist: create new user associated with business (secondary user first time auth)
        3. user and business both exist, user associated with business: simply update Google metadata for user
        4. user and business both exist, but user does not have this business: create associate, and update Google metadata for user (backfill - secondary user)

        Other scenarios are error state
    */

    businesses, businessAccountId, err := updateBusinesses(userId, userPtr, businessDao, google)
    if err != nil {
        log.Errorf("Error updating businesses: %s", err)

        lineSendErr := line.SendMessage(userId, "驗證失敗。請確認您有勾選授權智引力訪問您的商家訊息再重試！若已勾選，請聯繫客服。很抱歉為您造成不便。")
        if lineSendErr != nil {
            log.Errorf("Error sending LINE message to '%s': %s", userId, lineSendErr)
            metric.EmitLambdaMetric(enum4.Metric5xxError, enum2.HandlerNameAuthHandler.String(), 1)
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error updating businesses"}`,
        }, err
    }

    user, err := updateUser(userId, businesses, businessAccountId, userPtr, userDao, google, line)
    if err != nil {
        log.Errorf("Error updating user: %s", err)

        lineSendErr := line.SendMessage(userId, "驗證失敗。請確認您有勾選授權智引力訪問您的商家訊息再重試！若已勾選，請聯繫客服。很抱歉為您造成不便。")
        if lineSendErr != nil {
            log.Errorf("Error sending LINE message to '%s': %s", userId, lineSendErr)
            metric.EmitLambdaMetric(enum4.Metric5xxError, enum2.HandlerNameAuthHandler.String(), 1)
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error updating user"}`,
        }, err
    }

    // ----------------
    // Notify Slack channel of new business creation
    // ----------------
    err = slackUtil.NewSlack(log, stage).SendNewUserOauthCompletionMessage(user, businesses)
    if err != nil {
        log.Errorf("Error sending Slack message: %s", err)
        metric.EmitLambdaMetric(enum4.Metric5xxError, enum2.HandlerNameAuthHandler.String(), 1)
    }

    err = line.SendMessage(userId, "驗證成功。可以開始使用啦！")
    if err != nil {
        log.Errorf("Error sending LINE message to '%s': %s", userId, err)
        metric.EmitLambdaMetric(enum4.Metric5xxError, enum2.HandlerNameAuthHandler.String(), 1)
    }

    log.Info("Successfully finished lambda execution")

    return events.LambdaFunctionURLResponse{
        StatusCode: 200,
        Body:       "智引力驗證成功。可以關掉此頁面了！",
        Headers: map[string]string{
            "Content-Type": "text/plain; charset=utf-8",
        },
    }, nil
}

func buildUpdateTokenAttributeActions(token oauth2.Token) ([]dbModel.AttributeAction, error) {
    accessTokenAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "google.accessToken", token.AccessToken)
    if err != nil {
        return []dbModel.AttributeAction{}, err
    }
    accessTokenExpireAtAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "google.accessTokenExpireAt", token.Expiry)
    if err != nil {
        return []dbModel.AttributeAction{}, err
    }

    refreshTokenAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "google.refreshToken", token.RefreshToken)
    if err != nil {
        return []dbModel.AttributeAction{}, err
    }

    return []dbModel.AttributeAction{accessTokenAction, accessTokenExpireAtAction, refreshTokenAction}, nil
}

// TODO: [INT-91] Remove backfill logic once all users have been backfilled
// backfillBusinessAttributesFromUser in-place backfills business attributes from user
func backfillBusinessAttributesFromUser(business model2.Business, user model2.User) {
    business.BusinessDescription = user.BusinessDescription
    business.Keywords = user.Keywords
    if user.KeywordEnabled == nil {
        business.KeywordEnabled = false
    } else {
        business.KeywordEnabled = *user.KeywordEnabled
    }
    business.QuickReplyMessage = user.QuickReplyMessage
    if user.AutoQuickReplyEnabled == nil {
        business.AutoQuickReplyEnabled = false
    } else {
        business.AutoQuickReplyEnabled = *user.AutoQuickReplyEnabled
    }
}

// updateBusinesses updates businesses and returns the updated businesses and business account ID
func updateBusinesses(
    userId string,
    user *model2.User, // TODO: [INT-91] Remove backfill logic once all legacy users have completed OAUTH
    businessDao *ddbDao.BusinessDao,
    google *googleUtil.GoogleClient,
) ([]model2.Business, string, error) {
    // Google businesses have two portions: business accountID and business locationID

    accounts, err := google.ListBusinessAccounts()
    if err != nil {
        return []model2.Business{}, "", err
    }
    var businessAccount mybusinessaccountmanagement.Account
    switch len(accounts) {
    case 0:
        log.Warn("User has no Google business accounts")
        return []model2.Business{}, "", errors.New("user has no Google business accounts")
    case 1:
        businessAccount = accounts[0]
    default:
        log.Warn("User has multiple Google business accounts. Using the first one: ", jsonUtil.AnyToJson(accounts))
        metric.EmitMetricWithNamespace(enum2.MetricMultipleBusinessAccounts.String(), 1.0, util.AuthMetricNamespace)
        businessAccount = accounts[0]
    }

    businessLocations, err := google.ListBusinessLocations(businessAccount)
    if err != nil {
        return []model2.Business{}, "", err
    }

    businessAccountNameSlice := strings.Split(businessAccount.Name, "/")
    if len(businessAccountNameSlice) != 2 {
        log.Errorf("Error parsing business account ID %s", businessAccount.Name)
        return []model2.Business{}, "", errors.New("error parsing business account name")
    }
    businessAccountId := businessAccountNameSlice[1]

    businessLocations = googleUtil.FilterOpenBusinessLocations(businessLocations)
    if len(businessLocations) == 0 {
        log.Error("User has no open Google business locations under account ", businessAccountId)
        return []model2.Business{}, businessAccountId, errors.New("user has no open Google business locations")
    }
    if len(businessLocations) > 1 {
        log.Info("User has multiple open Google business locations.")
        metric.EmitMetricWithNamespace(enum2.MetricMultipleBusinessLocations.String(), 1.0, util.AuthMetricNamespace)
    }
    log.Info("User's open Google business locations are: ", jsonUtil.AnyToJson(businessLocations))

    var businesses []model2.Business
    for _, location := range businessLocations {
        businessLocationSlice := strings.Split(location.Name, "/")
        if len(businessLocationSlice) != 2 {
            log.Errorf("Error parsing business location ID %s", location.Name)
            return []model2.Business{}, businessAccountId, errors.New("error parsing business location name")
        }

        businessId, err := bid.NewBusinessId(businessLocationSlice[1])
        if err != nil {
            log.Errorf("Error creating businessId from business location.Name %s: %s", location.Name, err)
            return []model2.Business{}, businessAccountId, err
        }

        businessPtr, err := businessDao.GetBusiness(businessId)
        if err != nil {
            log.Errorf("Error retrieving business %s: %s", businessId, err)
            return []model2.Business{}, businessAccountId, err
        }

        var business model2.Business
        if businessPtr == nil {
            log.Infof("Business '%s' does not exist. Creating new business.", businessId)

            // create business
            business = model2.NewBusiness(
                businessId,
                location.Title,
                userId,
            )

            // TODO: [INT-91] Remove backfill logic once all legacy users have completed OAUTH
            if user != nil {
                backfillBusinessAttributesFromUser(business, *user)
            }

            err = businessDao.CreateBusiness(business)
            if err != nil {
                log.Errorf("Error creating business object %v: %v", business, err)
                return businesses, businessAccountId, err
            }
        } else {
            business = *businessPtr
            if !stringUtil.StringInSlice(userId, business.UserIds) {
                log.Infof("Business '%s' is unaware of '%s' yet. Creating association.", businessId, userId)

                // add user to business and update business Google token
                userIdAppendAction, err := dbModel.NewAttributeAction(enum.ActionAppendStringSet, "userIds", []string{userId})
                if err != nil {
                    log.Errorf("Error building user id append action: %s", err)
                    return businesses, businessAccountId, err
                }

                business, err = businessDao.UpdateAttributes(businessId, []dbModel.AttributeAction{userIdAppendAction}, userId)
            }
        }
        businesses = append(businesses, business)
    }

    return businesses, businessAccountId, nil
}

func updateUser(
    userId string,
    businesses []model2.Business,
    businessAccountId string,
    userPtr *model2.User,
    userDao *ddbDao.UserDao,
    google *googleUtil.GoogleClient,
    line *lineUtil.Line,
) (model2.User, error) {
    // get user info from Google
    googleUserInfo, err := google.GetGoogleUserInfo()
    if err != nil {
        log.Errorf("Error retrieving Google user info: %s", err)
        return model2.User{}, err
    }

    log.Debug("Google user info: ", jsonUtil.AnyToJson(googleUserInfo))

    googleMetadata := model2.Google{
        Id:                  googleUserInfo.Id,
        AccessToken:         google.Token.AccessToken,
        AccessTokenExpireAt: google.Token.Expiry,
        RefreshToken:        google.Token.RefreshToken,
        ProfileFullName:     googleUserInfo.Name,
        Email:               googleUserInfo.Email,
        ImageUrl:            googleUserInfo.Picture,
        Locale:              googleUserInfo.Locale,
        BusinessAccountId:   businessAccountId,
    }

    // extract business IDs from businesses
    var businessIds []bid.BusinessId
    for _, business := range businesses {
        businessIds = append(businessIds, business.BusinessId)
    }

    var user model2.User
    if userPtr == nil {
        log.Infof("User '%s' does not exist. Creating new user.", userId)

        lineGetUserResp, err := line.GetUser(userId)
        if err != nil {
            log.Errorf("Error retrieving user %s from LINE: %s", userId, err)
            return model2.User{}, err
        }

        user, err = model2.NewUser(userId, businessIds, lineGetUserResp, googleMetadata)
        if err != nil {
            log.Errorf("Error creating new user object: %s", err)
            return model2.User{}, err
        }

        err = userDao.CreateUser(user)
        if err != nil {
            log.Errorf("Error creating user %v: %v", user, err)

            var userAlreadyExistException *exception.UserAlreadyExistException
            if errors.As(err, &userAlreadyExistException) {
                log.Errorf("User %s already exists. Concurrency issue?", userId)
            }
            return user, err
        }
    } else {
        log.Infof("User %s already exists. Updating Google token and adding missing businessId associations", userId)

        user = *userPtr

        // build google metadata update action
        var actions []dbModel.AttributeAction
        var err error
        // TODO: [INT-91] Remove backfill logic once all users have completed googleMetadata migration
        if stringUtil.IsEmptyString(userPtr.Google.Id) {
            log.Infof("User %s does not have Google metadata. Creating.", userId)
            action, err := dbModel.NewAttributeAction(enum.ActionUpdate, "google", googleMetadata)
            if err != nil {
                log.Errorf("Error building update Google attribute action: %s", err)
                return model2.User{}, err
            }
            actions = []dbModel.AttributeAction{action}
        } else {
            actions, err = buildUpdateTokenAttributeActions(google.Token)
            if err != nil {
                log.Errorf("Error building update Google attribute action: %s", err)
                return model2.User{}, err
            }
            updateBusinessAccountIdAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "google.businessAccountId", businessAccountId)
            if err != nil {
                log.Errorf("Error building update business account ID attribute action: %s", err)
                return model2.User{}, err
            }
            actions = append(actions, updateBusinessAccountIdAction)
        }

        // find businessIds not in user's businessIds and add them
        var businessIdsMissingInUser []bid.BusinessId
        for _, businessId := range businessIds {
            if !stringUtil.StringInSlice(businessId.String(), bid.BusinessIdsToStringSlice(user.BusinessIds)) {
                businessIdsMissingInUser = append(businessIdsMissingInUser, businessId)
            }
        }
        if len(businessIdsMissingInUser) > 0 {
            for _, businessId := range businessIdsMissingInUser {
                action, err := dbModel.NewAttributeAction(enum.ActionAppendStringSet, "businessIds", []string{businessId.String()})
                if err != nil {
                    log.Errorf("Error building businessIds append action: %s", err)
                    return user, err
                }
                actions = append(actions, action)
            }
        }

        // repair active businessID if it is missing
        if stringUtil.IsEmptyString(user.ActiveBusinessId.String()) {
            log.Infof("User %s does not have active businessId. Repairing.", userId)
            action, err := dbModel.NewAttributeAction(enum.ActionUpdate, "activeBusinessId", businessIds[0].String())
            if err != nil {
                log.Errorf("Error building activeBusinessId update action: %s", err)
                return user, err
            }
            actions = append(actions, action)
        }

        // update user
        user, err = userDao.UpdateAttributes(userId, actions)
        if err != nil {
            log.Errorf("Error updating user %s: %s", userId, err)
            return user, err
        }
    }

    return user, nil
}
