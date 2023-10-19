package main

import (
    "context"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/dbModel"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/googleUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/lineUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/middleware"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "golang.org/x/oauth2"
    "os"
    "time"
)

func main() {
    lambda.Start(middleware.MetricMiddleware(enum2.HandlerNameAuthHandler.String(), handleRequest))
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
    // 1. Check if user exists
    // ----
    // DDB
    cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-northeast-1"))
    if err != nil {
        log.Error("Error loading AWS config: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error loading AWS config"}`, StatusCode: 500}, nil
    }
    businessDao := ddbDao.NewBusinessDao(dynamodb.NewFromConfig(cfg), log)
    userDao := ddbDao.NewUserDao(dynamodb.NewFromConfig(cfg), log)

    user, err := userDao.GetUser(userId)
    if err != nil {
        log.Error("Error checking if user exists: ", err)
        return events.LambdaFunctionURLResponse{Body: `{"message": "Error checking if user exists"}`, StatusCode: 500}, nil
    }

    google, err := googleUtil.NewGoogleWithAuthCode(log, code)
    if err != nil {
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error creating Google OAUTH client"}`,
        }, err
    }

    log.Debug("Google token: ", jsonUtil.AnyToJson(google.Token))

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
    // log.Debugf("Google business location: %s", jsonUtil.AnyToJson(location))
    businessId := accountId + "/" + location.Name

    business, err := businessDao.GetBusiness(businessId)
    if err != nil {
        log.Errorf("Error retrieving business %s: %s", businessId, err)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error retrieving business"}`,
        }, err
    }

    /*
       scenarios:
        1. Neither user nor business exist: create new user and associated business (primary user first time auth)
        2. user exists, but business does not exist: create new business for user (backfill - primary user)
        2. Business exists, but user does not exist: create new user associated with business (secondary user first time auth)
        3. user and business both exist, user have this business: update Google token for business
        4. user and business both exist, but user does not have this business: add user to business (backfill - secondary user)

        Other scenarios are error state
    */
    opsCompleted := false
    if user == nil {
        log.Infof("User %s does not exist. Creating new user.", userId)

        line := lineUtil.NewLine(log)
        lineGetUserResp, err := line.GetUser(userId)
        if err != nil {
            log.Errorf("Error retrieving user %s from LINE: %s", userId, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error retrieving user from LINE"}`,
            }, err
        }

        newUser := model.NewUser(userId, businessId, lineGetUserResp, time.Now())
        err = userDao.CreateUser(newUser)
        if err != nil {
            if _, ok := err.(*exception.UserAlreadyExistException); ok {
                log.Errorf("User %s already exists. Concurrency issue?", userId)
            } else {
                log.Error("Error creating user:", err)
            }
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       fmt.Sprintf(`{"error": "Failed to create user: %s"}`, err),
            }, nil
        }

        if business != nil {
            action, err := dbModel.NewAttributeAction(enum.ActionAppendStringSet, "userIds", []string{userId})
            if err != nil {
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error creating attribute action"}`,
                }, err
            }
            _, err = businessDao.UpdateAttributes(businessId, []dbModel.AttributeAction{action}, userId)
            if err != nil {
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error updating business"}`,
                }, err
            }
        }
        opsCompleted = true
    }

    if business == nil {
        log.Infof("Business '%s' does not exist. Creating new business.", businessId)

        // get user info from Google
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
        newBusiness := model.NewBusiness(
            accountId+"/"+location.Name,
            location.Title,
            model.Google{
                Id:                  userInfo.Id,
                AccessToken:         google.Token.AccessToken,
                AccessTokenExpireAt: google.Token.Expiry,
                RefreshToken:        google.Token.RefreshToken,
                ProfileFullName:     userInfo.Name,
                Email:               userInfo.Email,
                ImageUrl:            userInfo.Picture,
                Locale:              userInfo.Locale,
            },
            userId,
        )

        err = businessDao.CreateBusiness(newBusiness)
        if err != nil {
            log.Errorf("Error creating business object %v: %v", newBusiness, err)

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error creating business object"}`,
            }, err
        }

        // TODO: [INT-91] Remove backfill logic once all users have been backfilled
        if user != nil {
            log.Infof("User %s exists. Backfilling user %s to business %s", userId, userId, businessId)
            backfillBusinessFromUser(newBusiness, *user)

            // associate business with user
            businessIdUpdateAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "activeBusinessId", businessId)
            if err != nil {
                log.Errorf("Error building business id update action: %s", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error building business id update action"}`,
                }, err
            }
            _, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{businessIdUpdateAction})
            if err != nil {
                log.Errorf("Error updating user %s attributes %v: %s", userId, businessIdUpdateAction, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error updating user attributes"}`,
                }, err
            }
        }

        opsCompleted = true
    }

    if business != nil && user != nil {
        if util.StringInSlice(userId, business.UserIds) {
            log.Infof("User %s already has association with business %s. Updating OAUTH token only.", userId, businessId)

            actions, err := buildUpdateTokenAttributeActions(google.Token)
            if err != nil {
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error building update token attribute actions"}`,
                }, err
            }

            _, err = businessDao.UpdateAttributes(businessId, actions, userId)
            if err != nil {
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error updating business attributes"}`,
                }, err
            }
        } else {
            // TODO: [INT-91] Remove backfill logic once all users have been backfilled
            log.Infof("User %s does not have association with business %s yet. Creating association and refreshing OAUTH token.", userId, businessId)

            // add user to business and update business Google token
            userIdAppendAction, err := dbModel.NewAttributeAction(enum.ActionAppendStringSet, "userIds", []string{userId})
            if err != nil {
                log.Errorf("Error building user id append action: %s", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error building user id append action"}`,
                }, err
            }
            tokenActions, err := buildUpdateTokenAttributeActions(google.Token)
            if err != nil {
                log.Errorf("Error building update token attribute actions: %s", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error building update token attribute actions"}`,
                }, err
            }

            _, err = businessDao.UpdateAttributes(businessId, append(tokenActions, userIdAppendAction), userId)
            if err != nil {
                log.Errorf("Error updating business %s attributes %v: %s", businessId, append(tokenActions, userIdAppendAction), err)

                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error updating business attributes"}`,
                }, err
            }

            // add business to user
            businessIdUpdateAction, err := dbModel.NewAttributeAction(enum.ActionUpdate, "activeBusinessId", businessId)
            if err != nil {
                log.Errorf("Error building business id update action: %s", err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error building business id update action"}`,
                }, err
            }
            _, err = userDao.UpdateAttributes(userId, []dbModel.AttributeAction{businessIdUpdateAction})
            if err != nil {
                log.Errorf("Error updating user %s attributes %v: %s", userId, businessIdUpdateAction, err)
                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error updating user attributes"}`,
                }, err
            }
        }
        opsCompleted = true
    }

    if !opsCompleted {
        log.Errorf("Error associating user %s with business %s", userId, businessId)
        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"錯誤": "無法將此用戶與 google 商家建立關聯"}`,
            Headers: map[string]string{
                "Content-Type": "text/plain; charset=utf-8",
            },
        }, err
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
// backfillBusinessFromUser in-place backfills business attributes from user
func backfillBusinessFromUser(business model.Business, user model.User) {
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
