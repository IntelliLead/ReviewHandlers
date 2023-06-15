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

