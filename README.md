# ReviewHandlers
Collection of Lambda handlers to process Google reviews


## Manual lambda testing
1. Test the handler locally. Expect
    ```
    ❯ go run main.go
    2023/05/26 10:51:02 expected AWS Lambda environment variables [_LAMBDA_SERVER_PORT AWS_LAMBDA_RUNTIME_API] are not defined
    exit status 1
    ``` 
2. build zip `❯ GOOS=linux GOARCH=amd64 go build -o main main.go && zip main.zip main`
3. Upload via Lambda console
4. Make request and observe response in CloudWatch Logs/Metrics 

## CDK auto-lambda deployment to alpha
1. Get alpha credentials `../script/get-tmp-creds.sh`
   1. Requires `aws configure sso` for the account if you haven't
2. `npm run ls` to find the lambda stack name
3. `npm run deploy IntelliLead-ap-northeast-1-alpha-DeploymentStacks/IntelliLead-ap-northeast-1-alpha-Lambda` to deploy the lambda

## Updating LINE Rich Menu
1. Update the JSON in `src/pkg/jsonUtil/json/lineRichMenu/createRichMenuInput.json`. Increment the version name in the `name` field.
2. Choose the right channel access token for your environment
3. Create rich menu entity in LINE by running `https://api.line.me/v2/bot/richmenu` with the updated JSON as body. Record the `richMenuId` in the response.
   ```
   {
    "richMenuId": "richmenu-d2102f540de0e2a3c8c70d52ce622194"
   }
   ```
4. Upload Rich menu image associated with new Rich Menu entity by running `https://api-data.line.me/v2/bot/richmenu/{{richMenuId}}/content` with the (new) Rich Menu image as attachment. Verify response is empty 200.
5. Set the newly configured Rich Menu entity as default by running `https://api.line.me/v2/bot/user/all/richmenu/{{richMenuId}}`. Verify response is empty 200.
