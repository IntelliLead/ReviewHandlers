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
    "github.com/IntelliLead/ReviewHandlers/src/pkg/metric"
    enum3 "github.com/IntelliLead/ReviewHandlers/src/pkg/metric/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/middleware"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type/bid"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "golang.org/x/oauth2"
    "os"
)

func main() {
    lambda.Start(middleware.MetricMiddleware(enum2.HandlerNameAuthHandler, handleRequest))
}

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    log := logger.NewLogger()
    stage := os.Getenv(util.StageEnvKey)
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
            metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error creating Google OAUTH client"}`,
        }, err
    }

    user, err := userDao.GetUser(userId)
    if err != nil {
        log.Error("Error checking if user exists: ", err)

        err := line.SendMessage(userId, "驗證失敗。請稍後再試。很抱歉問您造成不便！")
        if err != nil {
            log.Errorf("Error sending LINE message to '%s': %s", userId, err)
            metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
        }

        return events.LambdaFunctionURLResponse{Body: `{"message": "Error checking if user exists"}`, StatusCode: 500}, err
    }

    /*
       scenarios:
        1. Neither user nor business exist: create new user and associated business (primary user first time auth)
        2. user exists, but business does not exist: create new business, associate with user, and create/update Google metadata for user (backfill - primary user)
        2. Business exists, but user does not exist: create new user associated with business (secondary user first time auth)
        3. user and business both exist, user associated with business: simply update Google metadata for user
        4. user and business both exist, but user does not have this business: create associate, and update Google metadata for user (backfill - secondary user)

        Other scenarios are error state
    */

    // ----
    // 3. Update Business
    // ----
    // retrieve business location (businessId)
    businessAccount, businessLocations, err := google.GetBusinessAccountAndLocations()
    if err != nil {
        err := line.SendMessage(userId, "驗證失敗。請確認您有勾選授權智引力訪問您的商家訊息再重試！")
        if err != nil {
            log.Errorf("Error sending LINE message to '%s': %s", userId, err)
            metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error retrieving Google business location"}`,
        }, err
    }
    businessAccountId := businessAccount.Name

    businessLocations = googleUtil.FilterOpenBusinessLocations(businessLocations)
    if len(businessLocations) == 0 {
        log.Error("User has no open Google business locations under account ", businessAccountId)

        err := line.SendMessage(userId, "驗證失敗。請確認您的 Google 賬戶內有至少一個 Google 商家。")
        if err != nil {
            log.Errorf("Error sending LINE message to '%s': %s", userId, err)
            metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 400,
            Body:       `{"error": "User has no Google business locations"}`,
        }, nil
    }

    if len(businessLocations) > 1 {
        log.Warn("User has multiple open Google business locations: ", jsonUtil.AnyToJson(businessLocations))
        metric.EmitMetric(enum3.MetricMultipleBusinessLocations, 1.0)
    }

    var businessIdsToAssociateUser []bid.BusinessId

    for _, location := range businessLocations {
        businessId, err := bid.NewBusinessId(businessAccountId + "/" + location.Name)
        if err != nil {
            log.Errorf("Error creating businessId from businessAccountId %s and location.Name %s: %s", businessAccountId, location.Name, err)
            err := line.SendMessage(userId, "驗證失敗。系統錯誤。請聯繫智引力客服。很抱歉為您造成不便。")
            if err != nil {
                log.Errorf("Error sending LINE message to '%s': %s", userId, err)
                metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
            }

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error creating businessId"}`,
            }, err
        }

        business, err := businessDao.GetBusiness(businessId)
        if err != nil {
            log.Errorf("Error retrieving business %s: %s", businessId, err)

            err := line.SendMessage(userId, "驗證失敗。系統錯誤。請聯繫智引力客服。很抱歉為您造成不便。")
            if err != nil {
                log.Errorf("Error sending LINE message to '%s': %s", userId, err)
                metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
            }

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error retrieving business"}`,
            }, err
        }
        if business == nil {
            log.Infof("Business '%s' does not exist. Creating new business.", businessId)

            businessIdsToAssociateUser = append(businessIdsToAssociateUser, businessId)

            // create business
            newBusiness := model.NewBusiness(
                businessId,
                location.Title,
                userId,
            )

            // TODO: [INT-91] Remove backfill logic once all users have been backfilled
            if user != nil {
                backfillBusinessFromUser(newBusiness, *user)
            }

            err = businessDao.CreateBusiness(newBusiness)
            if err != nil {
                log.Errorf("Error creating business object %v: %v", newBusiness, err)

                err := line.SendMessage(userId, "驗證失敗。系統錯誤。請聯繫智引力客服。很抱歉為您造成不便。")
                if err != nil {
                    log.Errorf("Error sending LINE message to '%s': %s", userId, err)
                    metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
                }

                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error creating business object"}`,
                }, err
            }
        } else {
            if !util.StringInSlice(userId, business.UserIds) {
                log.Infof("User %s does not have association with business %s yet. Creating association.", userId, businessId)

                businessIdsToAssociateUser = append(businessIdsToAssociateUser, businessId)

                // add user to business and update business Google token
                userIdAppendAction, err := dbModel.NewAttributeAction(enum.ActionAppendStringSet, "userIds", []string{userId})
                if err != nil {
                    log.Errorf("Error building user id append action: %s", err)

                    err := line.SendMessage(userId, "驗證失敗。系統錯誤。請聯繫智引力客服。很抱歉為您造成不便。")
                    if err != nil {
                        log.Errorf("Error sending LINE message to '%s': %s", userId, err)
                        metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
                    }

                    return events.LambdaFunctionURLResponse{
                        StatusCode: 500,
                        Body:       `{"error": "Error building user id append action"}`,
                    }, err
                }

                _, err = businessDao.UpdateAttributes(businessId, []dbModel.AttributeAction{userIdAppendAction}, userId)
            }
        }

    }

    // ----
    // 4. Update user
    // ----
    // get user info from Google
    googleUserInfo, err := google.GetGoogleUserInfo()
    if err != nil {
        log.Errorf("Error retrieving Google user info: %s", err)

        err := line.SendMessage(userId, "驗證失敗。系統錯誤。請確認您有授權智引力訪問您的 Google 賬戶信息。或請聯繫智引力客服。很抱歉為您造成不便。")
        if err != nil {
            log.Errorf("Error sending LINE message to '%s': %s", userId, err)
            metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
        }

        return events.LambdaFunctionURLResponse{
            StatusCode: 500,
            Body:       `{"error": "Error retrieving Google user info"}`,
        }, err
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
    }

    if user == nil {
        log.Infof("User %s does not exist. Creating new user.", userId)

        line := lineUtil.NewLine(log)
        lineGetUserResp, err := line.GetUser(userId)
        if err != nil {
            log.Errorf("Error retrieving user %s from LINE: %s", userId, err)

            err := line.SendMessage(userId, "驗證失敗。系統錯誤。請聯繫智引力客服。很抱歉為您造成不便。")
            if err != nil {
                log.Errorf("Error sending LINE message to '%s': %s", userId, err)
                metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
            }

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error retrieving user from LINE"}`,
            }, err
        }

        businessIds, err := googleUtil.MapBusinessIds(businessAccountId, businessLocations)
        if err != nil {
            log.Errorf("Error mapping businessIds with businessAccountId %s and businessLocations %s: %s", businessAccountId, businessLocations, err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error mapping businessIds with businessAccountId and businessLocations"}`,
            }, err
        }
        newUser, err := model.NewUser(userId, businessIds, lineGetUserResp, googleMetadata)
        if err != nil {
            log.Errorf("Error creating new user object: %s", err)
            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error creating new user object"}`,
            }, err
        }
        err = userDao.CreateUser(newUser)
        if err != nil {
            log.Errorf("Error creating user %v: %v", newUser, err)

            var userAlreadyExistException *exception.UserAlreadyExistException
            if errors.As(err, &userAlreadyExistException) {
                log.Errorf("User %s already exists. Concurrency issue?", userId)
            }

            err := line.SendMessage(userId, "驗證失敗。系統錯誤。請聯繫智引力客服。很抱歉為您造成不便。")
            if err != nil {
                log.Errorf("Error sending LINE message to '%s': %s", userId, err)
                metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
            }

            return events.LambdaFunctionURLResponse{
                StatusCode: 501,
                Body:       fmt.Sprintf(`{"error": "Failed to create user: %s"}`, err),
            }, err
        }
    } else {
        log.Infof("User %s already exists. Updating Google token.", userId)
        if len(businessIdsToAssociateUser) > 0 {
            log.Infof("Adding businessIds %s to user %s", businessIdsToAssociateUser, userId)
        }

        var actions []dbModel.AttributeAction
        var err error
        // TODO: [INT-91] Remove backfill logic once all users have completed googleMetadata migration
        if util.IsEmptyString(user.Google.Id) {
            log.Infof("User %s does not have Google metadata. Creating.", userId)
            action, _err := dbModel.NewAttributeAction(enum.ActionUpdate, "google", googleMetadata)
            err = _err
            actions = []dbModel.AttributeAction{action}
        } else {
            actions, err = buildUpdateTokenAttributeActions(google.Token)
        }
        if err != nil {
            err := line.SendMessage(userId, "驗證失敗。系統錯誤。請聯繫智引力客服。很抱歉為您造成不便。")
            if err != nil {
                log.Errorf("Error sending LINE message to '%s': %s", userId, err)
                metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
            }

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error building update token attribute actions"}`,
            }, err
        }

        for _, businessId := range businessIdsToAssociateUser {
            action, err := dbModel.NewAttributeAction(enum.ActionAppendStringSet, "businessIds", []string{businessId.String()})
            if err != nil {
                err := line.SendMessage(userId, "驗證失敗。系統錯誤。請聯繫智引力客服。很抱歉為您造成不便。")
                if err != nil {
                    log.Errorf("Error sending LINE message to '%s': %s", userId, err)
                    metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
                }

                return events.LambdaFunctionURLResponse{
                    StatusCode: 500,
                    Body:       `{"error": "Error creating attribute action"}`,
                }, err
            }
            actions = append(actions, action)
        }

        _, err = userDao.UpdateAttributes(userId, actions)
        if err != nil {
            err := line.SendMessage(userId, "驗證失敗。系統錯誤。請聯繫智引力客服。很抱歉為您造成不便。")
            if err != nil {
                log.Errorf("Error sending LINE message to '%s': %s", userId, err)
                metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
            }

            return events.LambdaFunctionURLResponse{
                StatusCode: 500,
                Body:       `{"error": "Error updating user attributes"}`,
            }, err
        }
    }

    log.Info("Successfully finished lambda execution")

    err = line.SendMessage(userId, "驗證成功。可以開始使用啦！")
    if err != nil {
        log.Errorf("Error sending LINE message to '%s': %s", userId, err)
        metric.EmitLambdaMetric(enum3.Metric5xxError, enum2.HandlerNameAuthHandler, 1)
    }

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
