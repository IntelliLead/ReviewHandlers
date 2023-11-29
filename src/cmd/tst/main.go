package main

import (
    "context"
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/dbModel"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/googleUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "golang.org/x/oauth2"
    "os"
    "time"
)

func main() {
    lambda.Start(handleRequest)
}

var (
    log = logger.NewLogger()
)

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    stage := os.Getenv(util.StageEnvKey)
    log.Infof("Received request in %s: %s", stage, jsonUtil.AnyToJson(request))

    // --------------------
    // initialize resources
    // --------------------
    cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-northeast-1"))
    if err != nil {
        log.Error("Error loading AWS config: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error loading AWS config"}`, StatusCode: 500}, nil
    }
    // DDB
    // businessDao := ddbDao.NewBusinessDao(dynamodb.NewFromConfig(cfg), log)
    // userDao := ddbDao.NewUserDao(dynamodb.NewFromConfig(cfg), log)
    // reviewDao := ddbDao.NewReviewDao(dynamodb.NewFromConfig(cfg), log)

    // Google
    // create time from string 2023-11-17T12:06:38.826881381Z
    expiryAt := time.Date(2023, 11, 17, 12, 6, 38, 826881381, time.UTC)
    google, err := googleUtil.NewGoogleWithToken(log, oauth2.Token{
        AccessToken:  "ya29.a0AfB_byCQenGB1lYCPiWLWZPTwe6DzYr4006VMNappkCYbL_tLNAQFBOSniZ_bImV0QYliK0BYneONE272X8oF0Q1QzOIIJGEF9EETFDZlGkPtG4YdFOmviVmt3UFRw9vvk0vSOn_J7SGjaV5GHDlmGhVKdFX2MFEEvmIaCgYKAc0SARASFQHGX2Mi-FNR6raASNTm62_d5r7xPA0171",
        TokenType:    "Bearer",
        RefreshToken: "1//0elbmmMfOm1VoCgYIARAAGA4SNwF-L9IrSJl9Df-0Mg6Gb-oKNz1GCrBvsJYJfaWmAPYtcmS8ymCPxfKiBfAHb9hWZgwE3pbFjnI",
        Expiry:       expiryAt,
    })

    // // LINE
    // line := lineUtil.NewLine(log)

    // --------------------
    // TODO: set lambda on a CloudwatchEvent timer
    // --------------------

    // --------------------
    // TODO: list all users with a Google token
    // --------------------

    // --------------------
    // TODO: check S3 to see if CSV for the current month exists
    // --------------------

    // --------------------
    // Get reviews for business
    // --------------------
    businessId, _ := bid.NewBusinessId("14282334389737307772") // mucurry
    // googleAccountId := "115452951270381035990"
    // loc, err := time.LoadLocation("Asia/Taipei")
    // if err != nil {
    //     log.Errorf("Error loading location: %s", err)
    //     return events.LambdaFunctionURLResponse{}, err
    // }
    month := 5
    // reviews, err := google.ListReviews(googleAccountId, businessId.String(), time.Date(2023, time.Month(month), 1, 0, 0, 0, 0, loc))
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{}, err
    // }
    //
    // csvBuffer, err := googleUtil.GoogleReviewsToCSV(reviews)
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{}, err
    // }

    performanceMetrics, err := google.ListPerformanceMetrics(businessId.String(), time.Date(2023, time.Month(month), 1, 0, 0, 0, 0, time.UTC))
    if err != nil {
        log.Errorf("Error listing performance metrics: %s", err)
        return events.LambdaFunctionURLResponse{}, err
    }
    csvBuffer, err := googleUtil.PerformanceMetricsToCSV(performanceMetrics)
    if err != nil {
        log.Errorf("Error converting to CSV: %s", err)
        return events.LambdaFunctionURLResponse{}, err
    }

    // --------------------
    // Upload CSV to S3
    // --------------------
    s3Client := s3.NewFromConfig(cfg)
    uploader := manager.NewUploader(s3Client)
    result, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
        Bucket: aws.String("intellilead-test-google-review-reporting-bucket"),
        // Key:    aws.String(fmt.Sprintf("%s/%d/reviews.csv", businessId.String(), month)),
        Key:  aws.String(fmt.Sprintf("%s/%d/performanceMetrics.csv", businessId.String(), month)),
        Body: csvBuffer,
    })
    if err != nil {
        return events.LambdaFunctionURLResponse{}, err
    }
    log.Infof("Successfully uploaded to %s", result.Location)

    // --------------------
    // Upload CSV to OpenAI and start Assistant
    // --------------------

    // --------------------
    // Get OpenAI Assistant response and send report
    // --------------------

    // ########### end of flow ###########
    // ########### end of flow ###########
    // ########### end of flow ###########

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
    // Get all business locations
    // --------------------
    // businessId := "accounts/108369004405812112145/locations/14318252560721528455"
    // business, err := businessDao.GetBusiness(businessId)
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //     }, err
    // }
    // if business == nil {
    //     log.Errorf("Business %s does not exist", businessId)
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 400,
    //     }, err
    // }
    //
    // log.Infof("Business retrieved is: %s", jsonUtil.AnyToJson(business))
    //
    // googleClient, err := googleUtil.NewGoogleWithToken(log, googleUtil.GoogleToToken(*business.Google))
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //     }, err
    // }
    //
    // accounts, err := googleClient.ListBusinessAccounts()
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //     }, err
    // }
    // locations, err := googleClient.ListBusinessLocations(accounts[0])
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //     }, err
    // }
    // log.Info("Locations retrieved: ", jsonUtil.AnyToJson(locations))

    // --------------------
    // Check auth
    // --------------------
    // const srcUserId = "U1de8edbae28c05ac8c7435bbd19485cb"     // 今遇良研
    // const sendingUserId = "Ucc29292b212e271132cee980c58e94eb" // Shawn - IL Internal
    // // const sendingUserId = "U6d5b2c34bbe084e22be8e30e68650992" // Jessie - IL Internal
    //
    //
    // line := lineUtil.NewLine(log)
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
    // line := lineUtil.NewLine(log)
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
    line *lineUtil.Line,
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

        lineGetUserResp, err := line.GetUser(userId)
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
        if util.IsEmptyString(userPtr.Google.Id) {
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
            if !util.StringInSlice(businessId.String(), bid.BusinessIdsToStringSlice(user.BusinessIds)) {
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
