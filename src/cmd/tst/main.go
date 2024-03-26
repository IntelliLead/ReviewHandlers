package main

import (
    "context"
    "errors"
    "github.com/IntelliLead/CoreCommonUtil/aws"
    "github.com/IntelliLead/CoreCommonUtil/constant"
    "github.com/IntelliLead/CoreCommonUtil/googleUtil"
    "github.com/IntelliLead/CoreCommonUtil/jsonUtil"
    "github.com/IntelliLead/CoreCommonUtil/logger"
    "github.com/IntelliLead/CoreCommonUtil/secretUtil"
    "github.com/IntelliLead/CoreCommonUtil/ssmUtil"
    "github.com/IntelliLead/CoreCommonUtil/stringUtil"
    "github.com/IntelliLead/CoreDataAccess/ddbDao"
    "github.com/IntelliLead/CoreDataAccess/ddbDao/dbModel"
    "github.com/IntelliLead/CoreDataAccess/ddbDao/enum"
    "github.com/IntelliLead/CoreDataAccess/exception"
    "github.com/IntelliLead/CoreDataAccess/model"
    "github.com/IntelliLead/CoreDataAccess/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "golang.org/x/oauth2"
    "os"
    "time"
)

func main() {
    lambda.Start(handleRequest)
}

var (
    log             = logger.NewLogger()
    awsConfig       = aws.DefaultAwsConfig()
    secrets         = secretUtil.NewSecretUtil(awsConfig, log).GetSecrets()
    authRedirectUrl = ssmUtil.NewSsm(awsConfig, log).GetSsmParameterValue(os.Getenv(constant.AuthRedirectUrlParameterNameEnvKey))
)

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    stage := os.Getenv(constant.StageEnvKey)
    log.Infof("Received request in %s: %s", stage, jsonUtil.AnyToJson(request))

    // --------------------
    // initialize resources
    // --------------------
    // DDB
    // cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-northeast-1"))
    // if err != nil {
    //     log.Error("Error loading AWS config: ", err)
    //     return events.LambdaFunctionURLResponse{Body: `{"message": "Error loading AWS config"}`, StatusCode: 500}, nil
    // }
    // businessDao := ddbDao.NewBusinessDao(dynamodb.NewFromConfig(cfg), log)
    // userDao := ddbDao.NewUserDao(dynamodb.NewFromConfig(cfg), log)
    // reviewDao := ddbDao.NewReviewDao(dynamodb.NewFromConfig(cfg), log)

    // Google
    // Location: 蒜香豆幹
    // create time from string 2023-12-11T11:03:05.820437781Z
    expiryAt := time.Date(2023, 12, 11, 11, 3, 5, 820437781, time.UTC)
    google, err := googleUtil.NewGoogleWithToken(
        authRedirectUrl,
        secrets.GoogleClientID,
        secrets.GoogleClientSecret,
        log,
        oauth2.Token{
            AccessToken:  "ya29.a0AfB_byDL6ceT7ponSPNtPdC3iZeZTr23Sjp8GNbAsZBuD5RjQTmRmMnUEYq-nBMigCKiXg1RWpP9sqfZd4UB7XkQRN15q1RpE7rtb0G9zIRLo3X2n8hKBXa6e5NWVsQVdcFh0jKsjTbUy995y-BLobX74hguabeVpxiJaCgYKAckSARISFQHGX2MigYiVeB6Q9I99JNAguTKS2A0171",
            TokenType:    "Bearer",
            RefreshToken: "1//0ep1o2HpKcyywCgYIARAAGA4SNwF-L9IrAyy-dC1L4LoDSUX1agZIfOca3D3sdc2n-ZOCjS5MWjD8ZjWqfv5mQPTIsRl7R0INwwg",
            Expiry:       expiryAt,
        })

    // LINE
    // line := lineUtil.NewLineUtil(secrets.LineChannelSecret, secrets.LineChannelAccessToken, log)

    // --------------------
    // Get all business locations
    // --------------------
    // businessId, _ := bid.NewBusinessId("184164883650001245")
    // businessPtr, err := businessDao.GetBusiness(businessId)
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //     }, err
    // }
    // if businessPtr == nil {
    //     log.Errorf("Business %s does not exist", businessId)
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 400,
    //     }, err
    // }
    //
    // log.Infof("Business retrieved is: %s", jsonUtil.AnyToJson(businessPtr))

    // accounts, err := google.ListBusinessAccounts()
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //     }, err
    // }
    // log.Info("Accounts retrieved: ", jsonUtil.AnyToJson(accounts))
    //
    // locations, err := google.ListBusinessLocations(accounts[0])
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //     }, err
    // }
    // log.Info("Locations retrieved: ", jsonUtil.AnyToJson(locations))

    locationId := "5746273854471781614"
    startDate := time.Now().AddDate(-2, 10, -5)
    log.Info("Start date: ", startDate)
    dailyPerformanceMetrics, err := google.ListDailyPerformanceMetrics(locationId, startDate)
    if err != nil {
        return events.LambdaFunctionURLResponse{}, err
    }
    log.Info("Daily performance metrics retrieved: ", jsonUtil.AnyToJson(dailyPerformanceMetrics))

    // --------------------
    // Add business to user during auth
    // --------------------
    // bid1, _ := bid.NewBusinessId("12251512170589559833") // IL
    // bid2, _ := bid.NewBusinessId("4496688115335717986")  // IL Internal
    // businessIds := []bid.BusinessId{bid1, bid2}
    // var businesses []model.Business
    // for _, businessId := range businessIds {
    //     business, err := businessDao.GetBusiness(businessId)
    //     if err != nil {
    //         return events.LambdaFunctionURLResponse{
    //             StatusCode: 500,
    //         }, err
    //     }
    //     if business == nil {
    //         log.Errorf("Business %s does not exist", businessId)
    //         return events.LambdaFunctionURLResponse{
    //             StatusCode: 400,
    //         }, err
    //     }
    //     businesses = append(businesses, *business)
    // }
    //
    // userId := "Ucc29292b212e271132cee980c58e94eb"
    //
    // businessAccountId := "106775638291982182570"
    //
    // // get user
    // userPtr, err := userDao.GetUser(userId)
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //     }, err
    // }
    //
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //     }, err
    // }
    //
    // user, err := updateUser(userId,
    //     businesses,
    //     businessAccountId,
    //     userPtr,
    //     userDao,
    //     google,
    //     line,
    // )
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{}, err
    // }
    //
    // log.Infof("User updated: %s", jsonUtil.AnyToJson(user))

    // --------------------
    // Check auth
    // --------------------
    // const srcUserId = "U1de8edbae28c05ac8c7435bbd19485cb"     // 今遇良研
    // const sendingUserId = "Ucc29292b212e271132cee980c58e94eb" // Shawn - IL Internal
    // // const sendingUserId = "U6d5b2c34bbe084e22be8e30e68650992" // Jessie - IL Internal
    //
    //
    // line := lineUtil.NewLineUtil(log)
    // hasUserAuthed, user, business, err := auth.ValidateUserAuthOrRequestAuthTst(
    //     sendingUserId,
    //     userDao,
    //     businessDao,
    //     line,
    //     log,
    // )
    // if err != nil {
    //     log.Errorf("Failed to validate user auth: %s", err.Error())
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //         Body:       `{"error": "Failed to validate user auth"}`,
    //     }, err
    // }
    //
    // log.Infof("User authed: %t", hasUserAuthed)
    // log.Infof("User: %s", jsonUtil.AnyToJson(user))
    // log.Infof("Business: %s", jsonUtil.AnyToJson(business))

    // --------------------
    // Send Auth Request
    // --------------------
    // line := lineUtil.NewLineUtil(log)
    //
    // // send auth request
    // const sendingUserId = "Ucc29292b212e271132cee980c58e94eb" // Shawn - IL Internal
    // err := line.SendAuthRequest(sendingUserId)
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //         Body:       `{"error": "Failed to send auth request"}`,
    //     }, err
    // }

    return events.LambdaFunctionURLResponse{Body: `{"message": "OK"}`, StatusCode: 200}, nil
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

func updateUser(
    userId string,
    businesses []model.Business,
    businessAccountId string,
    userPtr *model.User,
    userDao *ddbDao.UserDao,
    google *googleUtil.GoogleClient,
    line *lineUtil.LineUtil,
) (model.User, error) {
    // get user info from Google
    googleUserInfo, err := google.GetGoogleUserInfo()
    if err != nil {
        log.Errorf("Error retrieving Google user info: %s", err)
        return model.User{}, err
    }

    log.Debug("Google user info: ", jsonUtil.AnyToJson(googleUserInfo))

    googleMetadata := model.Google{
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

    var user model.User
    if userPtr == nil {
        log.Infof("User '%s' does not exist. Creating new user.", userId)

        lineGetUserResp, err := line.Base.GetUser(userId)
        if err != nil {
            log.Errorf("Error retrieving user %s from LINE: %s", userId, err)
            return model.User{}, err
        }

        user, err = model.NewUser(userId, businessIds, lineGetUserResp, googleMetadata)
        if err != nil {
            log.Errorf("Error creating new user object: %s", err)
            return model.User{}, err
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
                return model.User{}, err
            }
            actions = []dbModel.AttributeAction{action}
        } else {
            actions, err = buildUpdateTokenAttributeActions(google.Token)
            if err != nil {
                log.Errorf("Error building update Google attribute action: %s", err)
                return model.User{}, err
            }
            updateBusinessAccountIdAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "google.businessAccountId", businessAccountId)
            if err != nil {
                log.Errorf("Error building update business account ID attribute action: %s", err)
                return model.User{}, err
            }
            actions = append(actions, updateBusinessAccountIdAction)
        }

        // build add missing business IDs action
        // find businessIds not in user's businessIds
        var businessIdsToAssociateUser []bid.BusinessId
        for _, businessId := range businessIds {
            if !stringUtil.StringInSlice(businessId.String(), bid.BusinessIdsToStringSlice(user.BusinessIds)) {
                businessIdsToAssociateUser = append(businessIdsToAssociateUser, businessId)
            }
        }
        if len(businessIdsToAssociateUser) > 0 {
            for _, businessId := range businessIdsToAssociateUser {
                action, err := dbModel.NewAttributeAction(enum.ActionAppendStringSet, "businessIds", []string{businessId.String()})
                if err != nil {
                    log.Errorf("Error building businessIds append action: %s", err)
                    return user, err
                }
                actions = append(actions, action)
            }
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
