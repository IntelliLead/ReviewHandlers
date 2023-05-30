package main

import (
    "context"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/client"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
    "github.com/google/uuid"
)


func main() {
    lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    logger := logger.InitLogger()

    logger.Debug("Received request: ", request)
    logger.Debug("Received request header: ", request.Headers)
    logger.Debug("Received request body: ", request.Body)

    logger.Info("Received request in JSON form: ", util.AnyToJson(request))

    mySession := session.Must(session.NewSession())
    ddb := dynamodb.New(mySession, aws.NewConfig().WithRegion("ap-northeast-1"))


    // check if user exists
    // request.Body

    // forward to LINE by calling LINE messaging API



    return events.LambdaFunctionURLResponse{Body: `{"message": "OK"}`, StatusCode: 200}, nil

    //
    // // Create a new LINE Bot client
    // channelSecret := "a6064245795375fee1fb9cc2e4711447"
    // channelAccessToken := "0PWI55x6HFQ1WfHOBTddspgVTpTbFtFmy9ImN7NuYqScSz0mTFjYDqb9dA8TeRaUHNCrAWJ0x6yv4iJiMNrki4ZuYS4UhntFFtKma5tocBpgMcnD8+Kg0cTz3yoghq24QKmKp7R7OfoaTn4i/m7Y1AdB04t89/1O/w1cDnyilFU="
    // lineClient, err := linebot.New(channelSecret, channelAccessToken)
    // if err != nil {
    // 	logger.Fatal("cannot create new Line Client", err)
    // }
    //
    // lineUtil := lineUtil.NewLineUtil(lineClient, logger)
    //
    // httpRequest := convertToHttpRequest(request)
    // logger.Debug("wrapped HTTP request is: ", request)
    //
    // lineEvents, err := lineClient.ParseRequest(httpRequest)
    // if err != nil {
    // 	// Log and return an error response
    // 	logger.Error("Error parsing request:", err)
    // 	return events.LambdaFunctionURLResponse{
    // 		StatusCode: 400,
    // 		Body:       fmt.Sprintf(`{"error": "Failed to parse request: %s"}`, err),
    // 	}, nil
    // }
    //
    // // DEBUG
    // logger.Debug("JSONified prased events is: ", eventToJson(lineEvents))
    //
    // // Print the parsed events
    // for _, event := range lineEvents {
    // 	logger.Infof("Processing event: %#v\n", event)
    //
    // 	// Handle each event type
    // 	switch event.Type {
    // 	case linebot.EventTypeMessage:
    //
    // 		// DEBUG: test handle
    // 		response, err := lineUtil.SendQuickReply(event.ReplyToken)
    // 		if err != nil {
    // 			logger.Error("Error sending message:", err)
    // 			return events.LambdaFunctionURLResponse{
    // 				StatusCode: 500,
    // 				Body:       fmt.Sprintf(`{"error": "Failed to send message: %s"}`, err),
    // 			}, nil
    // 		}
    // 		logger.Debug("Response:", response)
    // 		// default case: just print
    // 	default:
    // 		logger.Info("Unhandled event type: ", event.Type)
    // 	}
    // }
    //
    // // Return a 200 OK response
    // return events.LambdaFunctionURLResponse{Body: `{"message": "OK"}`, StatusCode: 200}, nil
}

func lineUserIdExists(client *dynamodb.DynamoDB, userId string) (bool, error) {
    // Prepare the input parameters for the GetItem operation
    input := &dynamodb.GetItemInput{
        TableName: aws.String("User"),
        Key: map[string]dynamodb.AttributeValue{
            "userId": {
                S: aws.String(userId),
            },
        },
    }

    // Execute the GetItem operation
    result, err := client.GetItem(context.Background(), input)
    if err != nil {
        return false, err
    }

    // Check if the item exists in the response
    return len(result.Item) > 0, nil
}

func putUser(client *dynamodb.DynamoDB, user model.User) error {
    // Marshal the user object into a DynamoDB attribute value map
    av, err := dynamodbattribute.MarshalMap(user)
    if err != nil {
        return err
    }

    // Prepare the input parameters for the PutItem operation
    input := &dynamodb.PutItemInput{
        TableName: aws.String("User"),
        Item:      av,
    }

    // Execute the PutItem operation
    _, err = client.PutItem(input)
    return err
}
