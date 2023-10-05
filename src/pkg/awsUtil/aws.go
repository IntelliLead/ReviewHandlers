package awsUtil

import (
    "encoding/json"
    "errors"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/awsUtil/awsModel"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/ssm"
    "github.com/aws/aws-secretsmanager-caching-go/secretcache"
    "go.uber.org/zap"
    "log"
    "os"
)

type Aws struct {
    log         *zap.SugaredLogger
    secretCache *secretcache.Cache
}

func NewAws(logger *zap.SugaredLogger) *Aws {
    return &Aws{
        log: logger,
    }
}

func (a *Aws) GetAuthRedirectUrl() (string, error) {
    // TODO: [INT-84] use Lambda extension to cache and fetch auth redirect URL
    // retrieve from SSM parameter store
    authRedirectUrlParameterName := os.Getenv(util.AuthRedirectUrlParameterNameEnvKey)
    ssmClient := ssm.New(session.Must(session.NewSession()))
    response, err := ssmClient.GetParameter(&ssm.GetParameterInput{
        Name: &authRedirectUrlParameterName,
    })
    if err != nil {
        a.log.Error("Unable to retrieve auth redirect URL from SSM parameter store: ", err)
        return "", err
    }
    authRedirectUrl := *response.Parameter.Value
    if util.IsEmptyString(authRedirectUrl) {
        a.log.Error("Auth redirect URL is empty")
        return "", errors.New("auth redirect URL is empty")
    }

    return authRedirectUrl, nil
}

func (a *Aws) GetSecrets() awsModel.Secrets {
    if a.secretCache == nil {
        cache, err := secretcache.New()
        if err != nil {
            a.log.Fatal("Error creating secret cache during bootstrap: ", err)
        }
        a.secretCache = cache
    }

    result, err := a.secretCache.GetSecretString(secretName)
    if err != nil {
        log.Fatal("Error getting secrets during bootstrap: ", err)
    }

    var secret awsModel.Secrets
    err = json.Unmarshal([]byte(result), &secret)
    if err != nil {
        log.Fatal("Error unmarshalling secrets during bootstrap: ", err)
    }
    return secret
}

const secretName = "ReviewHandlers/secrets"
