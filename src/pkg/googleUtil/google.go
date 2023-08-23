package googleUtil

import (
    "context"
    "errors"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/secret"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/ssm"
    "go.uber.org/zap"
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    "google.golang.org/api/googleapi"
    "google.golang.org/api/mybusinessaccountmanagement/v1"
    "google.golang.org/api/mybusinessbusinessinformation/v1"
    googleOauth "google.golang.org/api/oauth2/v2"
    "google.golang.org/api/option"
    "os"
)

type Google struct {
    config oauth2.Config
    token  *oauth2.Token
    log    *zap.SugaredLogger
}

func NewGoogle(logger *zap.SugaredLogger) (*Google, error) {
    // TODO: [INT-84] use Lambda extension to cache and fetch auth redirect URL
    // retrieve from SSM parameter store
    authRedirectUrlParameterName := os.Getenv(util.AuthRedirectUrlParameterNameEnvKey)
    ssmClient := ssm.New(session.Must(session.NewSession()))
    response, err := ssmClient.GetParameter(&ssm.GetParameterInput{
        Name: &authRedirectUrlParameterName,
    })
    if err != nil {
        logger.Error("Unable to retrieve auth redirect URL from SSM parameter store: ", err)
        return &Google{}, err
    }

    authRedirectUrl := *response.Parameter.Value

    secrets := secret.GetSecrets()
    config := oauth2.Config{
        ClientID:     secrets.GoogleClientID,
        ClientSecret: secrets.GoogleClientSecret,
        RedirectURL:  authRedirectUrl,
        Scopes:       []string{"https://www.googleapis.com/auth/business.manage"},
        Endpoint:     google.Endpoint,
    }

    return &Google{
        config: config,
        log:    logger,
    }, nil
}

// ExchangeToken exchanges the code for a token
// side effect: sets the token on the Google struct
func (g *Google) ExchangeToken(code string) (oauth2.Token, error) {
    token, err := g.config.Exchange(context.Background(), code)
    if err != nil {
        g.log.Errorf("Error exchanging code for token: %s", err)
        return oauth2.Token{}, err
    }

    g.token = token

    return *token, nil
}

func (g *Google) GetGoogleUserInfo() (*googleOauth.Userinfo, error) {
    if g.token == nil {
        return nil, errors.New("token is not set. Call ExchangeToken first")
    }

    g.log.Debug("Getting Google user info with token: ", jsonUtil.AnyToJson(g.token))

    ctx := context.Background()
    googleOauthClient, err := googleOauth.NewService(ctx,
        option.WithTokenSource(g.config.TokenSource(ctx, g.token)))
    if err != nil {
        g.log.Errorf("Error creating Google OAUTH client in GetGoogleUserInfo(): %s", err)
        return nil, err
    }

    g.log.Debug("googleOauthClient is ", jsonUtil.AnyToJson(googleOauthClient))

    req := googleOauthClient.Userinfo.Get()
    g.log.Debug("googleOauthClient.Userinfo.Get() request is ", jsonUtil.AnyToJson(req))

    resp, err := req.Do()
    // resp, err := googleOauthClient.Userinfo.Get().Do()
    if err != nil {
        g.log.Errorf("Error getting Google user info in GetGoogleUserInfo(): %s", err)
        return nil, err
    }

    return resp, nil
}

// GetBusinessLocation retrieves the business location for the user and account ID
func (g *Google) GetBusinessLocation() (mybusinessbusinessinformation.Location, string, error) {
    if g.token == nil {
        return mybusinessbusinessinformation.Location{}, "", errors.New("token is not set. Call ExchangeToken first")
    }

    mybusinessaccountmanagementService, err := mybusinessaccountmanagement.NewService(context.Background(),
        option.WithTokenSource(g.config.TokenSource(context.Background(), g.token)))
    if err != nil {
        g.log.Error("Error creating Google business account management service client: ", err)
        return mybusinessbusinessinformation.Location{}, "", err
    }

    // resp, err := mybusinessaccountmanagementService.Accounts.List().Do()
    googleReq := mybusinessaccountmanagementService.Accounts.List()
    g.log.Debug("list accounts googleReq is ", jsonUtil.AnyToJson(googleReq))
    resp, err := googleReq.Do()
    if err != nil {
        g.log.Error("Error listing Google business accounts: ", err)
        g.log.Error("Error details: ", jsonUtil.AnyToJson(err))
        g.log.Error("response is ", jsonUtil.AnyToJson(resp))

        return mybusinessbusinessinformation.Location{}, "", err
    }
    accounts := resp.Accounts
    g.log.Info("Retrieved accounts: ", jsonUtil.AnyToJson(accounts))

    // TODO: [INT-89] add metrics to track frequency of multiple accounts and locations
    if len(accounts) > 1 {
        g.log.Warn("User has multiple Google business accounts. Using the first one")
    }
    if len(accounts) == 0 {
        g.log.Warn("User has no Google business accounts")
        return mybusinessbusinessinformation.Location{}, "", nil
    }

    businessInfoClient, err := mybusinessbusinessinformation.NewService(context.Background(), option.WithTokenSource(g.config.TokenSource(context.Background(), g.token)))

    accountId := accounts[0].Name
    g.log.Debug("Using resp.Accounts[0].Name for list locations request, it is ", accountId)
    locationsGoogleReq := businessInfoClient.Accounts.Locations.List(accountId)
    g.log.Debug("list locations googleReq is ", jsonUtil.AnyToJson(locationsGoogleReq))
    locationsResp, err := locationsGoogleReq.Do(googleapi.QueryParameter("readMask", "name,title,storeCode,languageCode,categories,labels,openInfo,relationshipData,serviceItems"))
    if err != nil {
        g.log.Error("Error listing Google business locations: ", err)
        g.log.Error("Error details: ", jsonUtil.AnyToJson(err))
        g.log.Error("response is ", jsonUtil.AnyToJson(locationsResp))
        return mybusinessbusinessinformation.Location{}, "", err
    }

    locations := locationsResp.Locations
    g.log.Debug("Retrieved locations: ", jsonUtil.AnyToJson(locations))

    // TODO: [INT-89] add metrics to track frequency of multiple accounts and locations
    if len(locations) > 1 {
        g.log.Warnf("User has multiple Google business locations %s. Using the first one", jsonUtil.AnyToJson(locations))
    }

    return *locationsResp.Locations[0], accountId, nil
}
