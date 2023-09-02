package main

import (
    "context"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/auth"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/dbModel"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "golang.org/x/oauth2"
    "os"
)

func main() {
    lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    log := logger.NewLogger()
    stage := os.Getenv(util.StageEnvKey)
    log.Infof("Received request in %s: %s", stage, jsonUtil.AnyToJson(request))

    const srcUserId = "U1de8edbae28c05ac8c7435bbd19485cb" // 今遇良研
    // const sendingUserId = "Ucc29292b212e271132cee980c58e94eb" // Shawn - IL Internal
    const sendingUserId = "U6d5b2c34bbe084e22be8e30e68650992" // Jessie - IL Internal

    auth.ValidateUserAuthOrRequestAuth()

    // --------------------
    // Send Auth Request
    // --------------------
    // line := lineUtil.NewLine(log)
    //
    // // send auth request
    // // Supplied via tst SAM template
    // authRedirectUrl := os.Getenv(util.AuthRedirectUrlEnvKey)
    // err := line.RequestAuth(sendingUserId, authRedirectUrl)
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //         Body:       `{"error": "Failed to send auth request"}`,
    //     }, err
    // }

    // --------------------
    // ???
    // --------------------
    // businessId := "accounts/106775638291982182570/locations/12251512170589559833"
    // const userId = "Ucc29292b212e271132cee980c58e94eb" // IL alpha
    //
    // mySession := session.Must(session.NewSession())
    // // userDao := ddbDao.NewUserDao(dynamodb.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1")), log)
    //
    // businessDao := ddbDao.NewBusinessDao(dynamodb.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1")), log)
    //
    // // business, err := businessDao.GetBusiness(businessId)
    // // if err != nil {
    // //     log.Errorf("Error retrieving business %s: %s", businessId, err)
    // //     return events.LambdaFunctionURLResponse{
    // //         StatusCode: 500,
    // //         Body:       `{"error": "Error retrieving business"}`,
    // //     }, err
    // // }
    // // if business == nil {
    // //     log.Infof("Business %s does not exist. Creating.", businessId)
    // // }
    // //
    // // log.Debugf("Business retrieved is: %s", jsonUtil.AnyToJson(business))
    //
    // // 2023-08-23T12:55:13.739856077Z to time.Time
    // t, _ := time.Parse(time.RFC3339, "2023-08-23T12:55:13.739856077Z")
    //
    // token := oauth2.Token{
    //     AccessToken:  "ya29.a0AfB_byAoPntQ3iUyE2SGQMQLjRvZ8aMONkSW43NQpVmgaDsJuM6geoCICF1vJDdRzTmowBgw0-WmUMWEWosxQ4_Xrx26xQJuodPnHH4EmjBjAK6QdHsywQdJYZ9YZDAFW_v3rxumJU8z371DeqhyXONubLrt4K3CSc9ingaCgYKAQcSARMSFQHsvYls3VMQWpWadTx0QV2Gg2rJsw0173",
    //     TokenType:    "Bearer",
    //     RefreshToken: "1//0e8LCyi7r68jICgYIARAAGA4SNwF-L9IrW_E1PrgIPyae3CPXBT0AaioJN-HQ6-j7qGpEQl0xg-KCyQi1LVFI-n9YRsYZZoRWwQU",
    //     Expiry:       t,
    // }
    //
    // actions, err := buildUpdateTokenAttributeActions(token)
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //         Body:       `{"error": "Error building update token attribute actions"}`,
    //     }, err
    // }
    // userIdAppendAction, err := dbModel.NewAttributeAction(enum.ActionAppendStringSet, "userIds", []string{userId})
    // if err != nil {
    //     log.Errorf("Error building user id append action: %s", err)
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //         Body:       `{"error": "Error building user id append action"}`,
    //     }, err
    // }
    //
    // _, err = businessDao.UpdateAttributes(businessId, append(actions, userIdAppendAction))
    // if err != nil {
    //     return events.LambdaFunctionURLResponse{
    //         StatusCode: 500,
    //         Body:       `{"error": "Error updating business attributes"}`,
    //     }, err
    // }
    // log.Debugf("Token updated successfully")

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
