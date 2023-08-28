package googleUtil

import (
    "context"
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
    Token  oauth2.Token
    log    *zap.SugaredLogger
}

func NewGoogleWithAuthCode(logger *zap.SugaredLogger, authCode string) (*Google, error) {
    googleClient, err := newGoogle(logger)
    if err != nil {
        return &Google{}, err
    }

    token, err := googleClient.exchangeToken(authCode)
    if err != nil {
        logger.Error("Unable to retrieve token from web: ", err)
        return &Google{}, err
    }

    googleClient.Token = token

    return googleClient, nil
}

func NewGoogleWithToken(logger *zap.SugaredLogger, token oauth2.Token) (*Google, error) {
    googleClient, err := newGoogle(logger)
    if err != nil {
        return &Google{}, err
    }

    googleClient.Token = token
    return googleClient, nil
}

func newGoogle(logger *zap.SugaredLogger) (*Google, error) {
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

// exchangeToken exchanges the authorization code for a token
func (g *Google) exchangeToken(code string) (oauth2.Token, error) {
    token, err := g.config.Exchange(context.Background(), code)
    if err != nil {
        g.log.Errorf("Error exchanging code for token: %s", err)
        return oauth2.Token{}, err
    }

    return *token, nil
}

func (g *Google) GetGoogleUserInfo() (*googleOauth.Userinfo, error) {
    ctx := context.Background()
    googleOauthClient, err := googleOauth.NewService(ctx,
        option.WithTokenSource(g.config.TokenSource(ctx, &g.Token)))
    if err != nil {
        g.log.Errorf("Error creating Google OAUTH client in GetGoogleUserInfo(): %s", err)
        return nil, err
    }

    req := googleOauthClient.Userinfo.Get()
    resp, err := req.Do()
    if err != nil {
        g.log.Errorf("Error getting Google user info in GetGoogleUserInfo() with request %s: %s", jsonUtil.AnyToJson(req), err)
        return nil, err
    }

    return resp, nil
}

// GetBusinessLocation retrieves the business location for the user and business account ID
func (g *Google) GetBusinessLocation() (mybusinessbusinessinformation.Location, string, error) {
    mybusinessaccountmanagementService, err := mybusinessaccountmanagement.NewService(context.Background(),
        option.WithTokenSource(g.config.TokenSource(context.Background(), &g.Token)))
    if err != nil {
        g.log.Error("Error creating Google business account management service client: ", err)
        return mybusinessbusinessinformation.Location{}, "", err
    }

    listAccountsReq := mybusinessaccountmanagementService.Accounts.List()
    resp, err := listAccountsReq.Do()
    if err != nil {
        g.log.Errorf("Error listing Google business accounts in GetBusinessLocation() with request %s: %s", jsonUtil.AnyToJson(listAccountsReq), err)
        return mybusinessbusinessinformation.Location{}, "", err
    }
    accounts := resp.Accounts
    // g.log.Debug("Retrieved accounts: ", jsonUtil.AnyToJson(accounts))

    if len(accounts) == 0 {
        g.log.Warn("User has no Google business accounts")
        return mybusinessbusinessinformation.Location{}, "", nil
    }

    // TODO: [INT-89] add metrics to track frequency of multiple accounts and locations
    if len(accounts) > 1 {
        g.log.Warn("User has multiple Google business accounts. Using the first one")
    }

    businessInfoClient, err := mybusinessbusinessinformation.NewService(context.Background(), option.WithTokenSource(g.config.TokenSource(context.Background(), &g.Token)))

    accountId := accounts[0].Name
    locationsGoogleReq := businessInfoClient.Accounts.Locations.List(accountId)
    locationsResp, err := locationsGoogleReq.Do(googleapi.QueryParameter("readMask", "name,title,storeCode,languageCode,categories,labels,openInfo,relationshipData"))
    if err != nil {
        g.log.Errorf("Error listing Google business locations in GetBusinessLocation() with request %s: %s", jsonUtil.AnyToJson(locationsGoogleReq), err)
        return mybusinessbusinessinformation.Location{}, "", err
    }

    locations := locationsResp.Locations
    // g.log.Debug("Retrieved locations: ", jsonUtil.AnyToJson(locations))

    if len(locations) == 0 {
        g.log.Warn("User has no Google business locations under account ", accountId)
        return mybusinessbusinessinformation.Location{}, accountId, nil
    }

    // TODO: [INT-89] add metrics to track frequency of multiple accounts and locations
    if len(locations) > 1 {
        g.log.Warnf("User has multiple Google business locations. Using the first one %s", jsonUtil.AnyToJson(locations[0]))
    }

    return *locations[0], accountId, nil
}
