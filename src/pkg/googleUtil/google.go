package googleUtil

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/awsUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/metric"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/metric/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "go.uber.org/zap"
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    "google.golang.org/api/businessprofileperformance/v1"
    "google.golang.org/api/googleapi"
    "google.golang.org/api/mybusinessaccountmanagement/v1"
    "google.golang.org/api/mybusinessbusinessinformation/v1"
    googleOauth "google.golang.org/api/oauth2/v2"
    "google.golang.org/api/option"
    "io"
    "net/http"
    "time"
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
        g.log.Errorf("Error getting Google user info in GetGoogleUserInfo() with request %s: %s", jsonUtil.AnyToJson(req), err)
        return nil, err
    }

    return resp, nil
}

// GetBusinessAccountAndLocations retrieves the business location for the user and business account ID
func (g *GoogleClient) GetBusinessAccountAndLocations() (mybusinessaccountmanagement.Account, []mybusinessbusinessinformation.Location, error) {
    accounts, err := g.ListBusinessAccounts()
    if err != nil {
        g.log.Errorf("Error listing Google business accounts in GetBusinessAccountAndLocations(): %s", err)
        return mybusinessaccountmanagement.Account{}, []mybusinessbusinessinformation.Location{}, err
    }
    var account mybusinessaccountmanagement.Account
    switch len(accounts) {
    case 0:
        g.log.Warn("User has no Google business accounts")
        return mybusinessaccountmanagement.Account{}, []mybusinessbusinessinformation.Location{}, err
    case 1:
        account = accounts[0]
    default:
        g.log.Warn("User has multiple Google business accounts. Using the first one: ", jsonUtil.AnyToJson(accounts))
        metric.EmitMetric(enum2.MetricMultipleBusinessAccounts, 1.0)
        account = accounts[0]
    }

    locations, err := g.ListBusinessLocations(account)
    if err != nil {
        return mybusinessaccountmanagement.Account{}, []mybusinessbusinessinformation.Location{}, err
    }

    return account, locations, nil
}

func (g *GoogleClient) ListBusinessLocations(account mybusinessaccountmanagement.Account) ([]mybusinessbusinessinformation.Location, error) {
    businessInfoClient, err := mybusinessbusinessinformation.NewService(context.Background(), option.WithTokenSource(g.config.TokenSource(context.Background(), &g.Token)))
    if err != nil {
        g.log.Errorf("Error creating Google business information service client in GetBusinessAccountAndLocations(): %s", err)
        return []mybusinessbusinessinformation.Location{}, err
    }

    accountId := account.Name
    locationsGoogleReq := businessInfoClient.Accounts.Locations.List(accountId)
    locationsResp, err := locationsGoogleReq.Do(googleapi.QueryParameter("readMask", "name,title,storeCode,languageCode,categories,labels,openInfo,profile,serviceArea,serviceItems,storeCode,storefrontAddress"))
    if err != nil {
        g.log.Errorf("Error listing Google business locations in GetBusinessAccountAndLocations() with request %s: %s", jsonUtil.AnyToJson(locationsGoogleReq), err)
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
        g.log.Errorf("Error listing Google business accounts in GetBusinessAccountAndLocations() with request %s: %s", jsonUtil.AnyToJson(listAccountsReq), err)
    }

    g.log.Debug("Retrieved accounts: ", jsonUtil.AnyToJson(resp.Accounts))

    accounts := make([]mybusinessaccountmanagement.Account, len(resp.Accounts))
    for i, p := range resp.Accounts {
        accounts[i] = *p
    }

    return accounts, nil
}

// ListReviews retrieves all reviews for a given location, from current time to the earliestDate
func (g *GoogleClient) ListReviews(account string, location string, earliestDate time.Time) ([]GoogleReview, error) {
    client := g.config.Client(context.Background(), &g.Token)

    var allReviews []GoogleReview
    var pageToken string

    for {
        // 50 is the max page size: https://developers.google.com/my-business/reference/rest/v4/accounts.locations.reviews/list
        // default sort order is updateTime desc
        url := fmt.Sprintf("https://mybusiness.googleapis.com/v4/accounts/%s/locations/%s/reviews?pageSize=50", account, location)
        if pageToken != "" {
            url += "&pageToken=" + pageToken
        }

        httpResp, err := client.Get(url)
        if err != nil {
            g.log.Errorf("Error making request to Google API: %s", err)
            return nil, err
        }
        defer httpResp.Body.Close()
        if httpResp.StatusCode != http.StatusOK {
            g.log.Errorf("Non-OK HTTP status: %d", httpResp.StatusCode)
            return nil, fmt.Errorf("received non-OK status code: %d", httpResp.StatusCode)
        }
        body, err := io.ReadAll(httpResp.Body)
        if err != nil {
            g.log.Errorf("Error reading response body: %s", err)
            return nil, err
        }

        var response GoogleReviewsResponse
        err = json.Unmarshal(body, &response)
        if err != nil {
            g.log.Errorf("Error unmarshalling Google business reviews response body in ListReviews() with request: %s", err)
        }

        for idx, review := range response.Reviews {
            if review.UpdateTime.Before(earliestDate) {
                allReviews = append(allReviews, response.Reviews[:idx]...)
                return allReviews, nil
            }
        }
        allReviews = append(allReviews, response.Reviews...)

        if response.NextPageToken == "" {
            break
        }
        pageToken = response.NextPageToken
    }

    return allReviews, nil
}

func (g *GoogleClient) ListPerformanceMetrics(businessId string, earliestDate time.Time) ([]PerformanceMetric, error) {
    client, err := businessprofileperformance.NewService(context.Background(),
        option.WithTokenSource(g.config.TokenSource(context.Background(), &g.Token)))
    if err != nil {
        g.log.Error("Error creating Google business profile performance service client: ", err)
        return nil, err
    }

    now := time.Now()
    response, err := client.Locations.FetchMultiDailyMetricsTimeSeries(fmt.Sprintf("locations/%s", businessId)).
        DailyMetrics(DailyMetric.Strings(0)...). // exclude DailyMetricUnknown
        DailyRangeStartDateDay(int64(earliestDate.Day())).
        DailyRangeStartDateMonth(int64(earliestDate.Month())).
        DailyRangeStartDateYear(int64(earliestDate.Year())).
        DailyRangeEndDateDay(int64(now.Day())).
        DailyRangeEndDateMonth(int64(now.Month())).
        DailyRangeEndDateYear(int64(now.Year())).
        Do()
    if err != nil {
        g.log.Errorf("Error getting Google business profile performance metrics in ListPerformanceMetrics(): %s", err)
        return nil, err
    }

    metricsByDate := make(map[time.Time]map[DailyMetric]int, len(response.MultiDailyMetricTimeSeries[0].DailyMetricTimeSeries[0].TimeSeries.DatedValues))
    for _, timeSeries := range response.MultiDailyMetricTimeSeries[0].DailyMetricTimeSeries {
        dailyMetric, err := ToDailyMetric(timeSeries.DailyMetric)
        if err != nil {
            g.log.Errorf("Error converting daily metric in ListPerformanceMetrics(): %s", err)
            return nil, err
        }

        for _, point := range timeSeries.TimeSeries.DatedValues {
            date := time.Date(int(point.Date.Year), time.Month(point.Date.Month), int(point.Date.Day), 0, 0, 0, 0, time.UTC)
            if metricsByDate[date] == nil {
                metricsByDate[date] = make(map[DailyMetric]int)
            }

            metricsByDate[date][dailyMetric] = int(point.Value)
        }
    }

    var performanceMetrics []PerformanceMetric
    for date, metrics := range metricsByDate {
        performanceMetrics = append(performanceMetrics, PerformanceMetric{
            Date:    date,
            Metrics: metrics,
        })
    }

    return performanceMetrics, nil
}
