package main

import (
    "context"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/dbModel"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/googleUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "golang.org/x/oauth2"
    "os"
)

func main() {
    lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    ctx = context.Background()

    log := logger.NewLogger()
    stage := os.Getenv(util.StageEnvKey)
    log.Infof("Received request in %s: %s", stage, jsonUtil.AnyToJson(request))

    // const srcUserId = "U1de8edbae28c05ac8c7435bbd19485cb"     // 今遇良研
    // const sendingUserId = "Ucc29292b212e271132cee980c58e94eb" // IL alpha

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
    if code == "" {
        log.Errorf("Missing state parameter from Google OAUTH response")
        return events.LambdaFunctionURLResponse{
            StatusCode: 400,
            Body:       `{"error": "Missing state parameter from Google OAUTH response"}`,
        }, nil
    }

    log.Info("Received OAUTH request from user: ", userId)

    // ----
    // 0. Check if user exists
    // ----
    mySession := session.Must(session.NewSession())
    userDao := ddbDao.NewUserDao(dynamodb.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1")), log)
    businessDao := ddbDao.NewBusinessDao(dynamodb.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1")), log)

    isUserExist, user, err := userDao.IsUserExist(userId)
    if err != nil {
        log.Error("Error checking if user exists: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error checking if user exists"}`, StatusCode: 500}, nil
    }
    if !isUserExist {
        log.Error("User does not exist: ", userId)
        return events.LambdaFunctionURLResponse{Body: `{"message": "User does not exist"}`, StatusCode: 400}, nil
    }

    google, err := googleUtil.NewGoogle(log)
    if err != nil {
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error creating Google OAUTH client"}`,
        }, err
    }

    // ----
    // 1. exchange code for token
    // ----
    token, err := google.ExchangeToken(code)
    if err != nil {
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error exchanging code for token"}`,
        }, err
    }

    log.Debug("Google token: ", jsonUtil.AnyToJson(token))

    // ----
    // 2. Check if business exists
    // ----
    // retrieve business location (businessId)
    location, accountId, err := google.GetBusinessLocation()
    if err != nil {
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error retrieving Google business location"}`,
        }, err
    }
    log.Debugf("Google business location: %s", jsonUtil.AnyToJson(location))
    businessId := accountId + "/" + location.Name

    business, err := businessDao.GetBusiness(businessId)
    if err != nil {
        log.Errorf("Error retrieving business %s: %s", businessId, err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error retrieving business"}`,
        }, err
    }
    if business == nil {
        log.Infof("Business %s does not exist. Creating.", businessId)
    }

    /*
       scenarios:
        1. user have this business and business exists: update business Google token
        2. user does not have this business and business exists: add user to business (secondary user or not first time login)
        3. user does not have this business and business does not exist: create new business for user (primary user)

        Other scenarios are error state
    */

    if business != nil && util.StringInSlice(userId, business.UserIds) && util.StringInSlice(businessId, user.BusinessIds) {
        log.Infof("User %s already has association with business %s. Updating OAUTH token only.", userId, businessId)

        actions, err := buildUpdateTokenAttributeActions(token)
        if err != nil {
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error building update token attribute actions"}`,
            }, err
        }

        _, err = businessDao.UpdateAttributes(businessId, actions)
        if err != nil {
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error updating business attributes"}`,
            }, err
        }

    } else if business != nil && !util.StringInSlice(businessId, user.BusinessIds) {
        log.Infof("User %s does not have association with business %s yet. Adding user to business.", userId, businessId)

        // add user to business and update business Google token
        userIdAppendAction, err := dbModel.NewAttributeAction(enum.ActionAppend, "userIds", []string{userId})
        if err != nil {
            log.Errorf("Error building user id append action: %s", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error building user id append action"}`,
            }, err
        }
        tokenActions, err := buildUpdateTokenAttributeActions(token)
        if err != nil {
            log.Errorf("Error building update token attribute actions: %s", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error building update token attribute actions"}`,
            }, err
        }

        _, err = businessDao.UpdateAttributes(businessId, append(tokenActions, userIdAppendAction))
        if err != nil {
            log.Errorf("Error updating business %s attributes %v: %s", businessId, append(tokenActions, userIdAppendAction), err)

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error updating business attributes"}`,
            }, err
        }

        // add business to user
        businessIdAppendAction, err := dbModel.NewAttributeAction(enum.ActionAppend, "businessIds", []string{businessId})
        if err != nil {
            log.Errorf("Error building business id append action: %s", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error building business id append action"}`,
            }, err
        }
        _, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{businessIdAppendAction})
        if err != nil {
            log.Errorf("Error updating user %s attributes %v: %s", userId, businessIdAppendAction, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error updating user attributes"}`,
            }, err
        }

    } else if business == nil && !util.StringInSlice(businessId, user.BusinessIds) {
        log.Infof("Business %s does not exist. Creating and associating with user %s.", businessId, userId)

        // get user info from Google and create business
        userInfo, err := google.GetGoogleUserInfo()
        if err != nil {
            log.Errorf("Error retrieving Google user info: %s", err)

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error retrieving Google user info"}`,
            }, err
        }

        log.Debug("Google user info: ", jsonUtil.AnyToJson(userInfo))

        // create business
        business := model.NewBusiness(
            accountId+"/"+location.Name,
            location.Title,
            model.Google{
                Id:                  userInfo.Id,
                AccessToken:         token.AccessToken,
                AccessTokenExpireAt: token.Expiry,
                RefreshToken:        token.RefreshToken,
                ProfileFullName:     userInfo.Name,
                Email:               userInfo.Email,
                ImageUrl:            userInfo.Picture,
                Locale:              userInfo.Locale,
            },
            userId,
        )
        err = businessDao.CreateBusiness(business)
        if err != nil {
            log.Errorf("Error creating business object %v: %v", business, err)

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error creating business object"}`,
            }, err
        }

        // associate business with user
        businessIdAppendAction, err := dbModel.NewAttributeAction(enum.ActionAppend, "businessIds", []string{businessId})
        if err != nil {
            log.Errorf("Error building business id append action: %s", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error building business id append action"}`,
            }, err
        }
        _, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{businessIdAppendAction})
        if err != nil {
            log.Errorf("Error updating user %s attributes %v: %s", userId, businessIdAppendAction, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error updating user attributes"}`,
            }, err
        }

    } else {
        // error states
        log.Errorf("Error associating user %s with business %s", userId, businessId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error associating user with business"}`,
        }, err
    }

    log.Info("Successfully finished lambda execution")

    return events.LambdaFunctionURLResponse{Body: "智引力驗證成功。可以關掉此頁面了！", StatusCode: 200}, nil
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
