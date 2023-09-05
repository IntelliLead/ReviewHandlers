package secret

import (
    "encoding/json"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/secret/secretModel"
    "github.com/aws/aws-secretsmanager-caching-go/secretcache"
    "log"
)

func GetSecrets() secretModel.Secrets {
    secretCache, err := secretcache.New()
    if err != nil {
        log.Fatal("Error creating secret cache during bootstrap: ", err)
    }
    result, err := secretCache.GetSecretString(secretName)
    if err != nil {
        log.Fatal("Error getting secrets during bootstrap: ", err)
    }

    var secret secretModel.Secrets
    err = json.Unmarshal([]byte(result), &secret)
    if err != nil {
        log.Fatal("Error unmarshalling secrets during bootstrap: ", err)
    }
    return secret
}

const secretName = "ReviewHandlers/secrets"
