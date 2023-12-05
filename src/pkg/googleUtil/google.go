package googleUtil

import (
    "context"
    jsonUtil2 "github.com/IntelliLead/CoreCommonUtil/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/awsUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "go.uber.org/zap"
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    "google.golang.org/api/googleapi"
    "google.golang.org/api/mybusinessaccountmanagement/v1"
    "google.golang.org/api/mybusinessbusinessinformation/v1"
    googleOauth "google.golang.org/api/oauth2/v2"
    "google.golang.org/api/option"
)

type GoogleClient struct {
    config oauth2.Config
    Token  oauth2.Token
    log    *zap.SugaredLogger
}

func NewGoogleWithAuthCode(logger *zap.SugaredLogger, authCode string) (*GoogleClient, error) {
    googleClient, err := newGoogle(logger)
    if err != nil {
        return &GoogleClient{}, err
    }

    // local testing
    if authCode == util.TestAuthCode {
        return googleClient, nil
    }

    token, err := googleClient.exchangeToken(authCode)
    if err != nil {
        logger.Error("Unable to retrieve token from web: ", err)
        return &GoogleClient{}, err
    }

    googleClient.Token = token

    return googleClient, nil
}

func NewGoogleWithToken(logger *zap.SugaredLogger, token oauth2.Token) (*GoogleClient, error) {
    googleClient, err := newGoogle(logger)
    if err != nil {
        return &GoogleClient{}, err
    }

    googleClient.Token = token
    return googleClient, nil
}

func newGoogle(logger *zap.SugaredLogger) (*GoogleClient, error) {
    aws := awsUtil.NewAws(logger)
    authRedirectUrl, _ := aws.GetAuthRedirectUrl()

    secrets := aws.GetSecrets()
    config := oauth2.Config{
        ClientID:     secrets.GoogleClientID,
        ClientSecret: secrets.GoogleClientSecret,
        RedirectURL:  authRedirectUrl,
        Scopes:       []string{"https://www.googleapis.com/auth/business.manage"},
        Endpoint:     google.Endpoint,
    }

    return &GoogleClient{
        config: config,
        log:    logger,
    }, nil
}

// exchangeToken exchanges the authorization code for a token
func (g *GoogleClient) exchangeToken(code string) (oauth2.Token, error) {
    token, err := g.config.Exchange(context.Background(), code)
    if err != nil {
        g.log.Errorf("Error exchanging code for token: %s", err)
        return oauth2.Token{}, err
    }

    return *token, nil
}

func (g *GoogleClient) GetGoogleUserInfo() (*googleOauth.Userinfo, error) {
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
        g.log.Errorf("Error getting Google user info in GetGoogleUserInfo() with request %s: %s", jsonUtil2.AnyToJson(req), err)
        return nil, err
    }

    return resp, nil
}

func (g *GoogleClient) ListBusinessLocations(account mybusinessaccountmanagement.Account) ([]mybusinessbusinessinformation.Location, error) {
    businessInfoClient, err := mybusinessbusinessinformation.NewService(context.Background(), option.WithTokenSource(g.config.TokenSource(context.Background(), &g.Token)))
    accountId := account.Name
    locationsGoogleReq := businessInfoClient.Accounts.Locations.List(accountId)
    locationsResp, err := locationsGoogleReq.Do(googleapi.QueryParameter("readMask", "name,title,storeCode,languageCode,categories,labels,openInfo,profile,serviceArea,serviceItems,storeCode,storefrontAddress"))
    if err != nil {
        g.log.Errorf("Error listing Google business locations in GetBusinessAccountAndLocations() with request %s: %s", jsonUtil2.AnyToJson(locationsGoogleReq), err)
        return []mybusinessbusinessinformation.Location{}, err
    }

    locations := make([]mybusinessbusinessinformation.Location, len(locationsResp.Locations))
    for i, p := range locationsResp.Locations {
        locations[i] = *p
    }

    return locations, nil
}

func (g *GoogleClient) ListBusinessAccounts() ([]mybusinessaccountmanagement.Account, error) {
    mybusinessaccountmanagementService, err := mybusinessaccountmanagement.NewService(context.Background(),
        option.WithTokenSource(g.config.TokenSource(context.Background(), &g.Token)))
    if err != nil {
        g.log.Error("Error creating Google business account management service client: ", err)
    }

    listAccountsReq := mybusinessaccountmanagementService.Accounts.List()
    resp, err := listAccountsReq.Do()
    if err != nil {
        g.log.Errorf("Error listing Google business accounts in GetBusinessAccountAndLocations() with request %s: %s", jsonUtil2.AnyToJson(listAccountsReq), err)
    }

    g.log.Debug("Retrieved accounts: ", jsonUtil2.AnyToJson(resp.Accounts))

    accounts := make([]mybusinessaccountmanagement.Account, len(resp.Accounts))
    for i, p := range resp.Accounts {
        accounts[i] = *p
    }

    return accounts, nil
}
