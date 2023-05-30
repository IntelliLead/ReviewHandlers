# ReviewHandlers
Collection of Lambda handlers to process Google reviews


## Manual lambda testing
1. Test the handler locally. Expect
    ```
    ❯ go run main.go
    2023/05/26 10:51:02 expected AWS Lambda environment variables [_LAMBDA_SERVER_PORT AWS_LAMBDA_RUNTIME_API] are not defined
    exit status 1
    ``` 
4. build zip `❯ GOOS=linux GOARCH=amd64 go build -o main main.go && zip main.zip main`
5. Upload via Lambda console
6. Make request and observe response in CloudWatch Logs/Metrics 