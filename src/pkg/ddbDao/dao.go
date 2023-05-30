package ddbDao

import (
	"fmt"
	"github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
	"github.com/IntelliLead/ReviewHandlers/src/pkg/model"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"go.uber.org/zap"
	"strconv"
)

type Dao struct {
	ddbClient *dynamodb.DynamoDB
	logger    *zap.SugaredLogger
}

func NewDao(client *dynamodb.DynamoDB, logger *zap.SugaredLogger) *Dao {
	return &Dao{
		ddbClient: client,
		logger:    logger,
	}
}

// func (d *Dao) IsUserIdExist(userId string) (bool, error) {
// 	// Prepare the input parameters for the GetItem operation
// 	input := &dynamodb.GetItemInput{
// 		TableName: aws.String("User"),
// 		Key: map[string]dynamodb.AttributeValue{
// 			"lineUserId": {
// 				S: aws.String(userId),
// 			},
// 		},
// 	}
//
// 	// Execute the GetItem operation
// 	result, err := d.ddbClient.GetItem(input)
// 	if err != nil {
// 		return false, err
// 	}
//
// 	// Check if the item exists in the response
// 	return len(result.Item) > 0, nil
// }

// CreateUser creates a new user in the User table
// error handling
// 1. user already exist UserAlreadyExistException
// 2. aws error
func (d *Dao) CreateUser(user model.User) error {
	av, err := userMarshalMap(user)
	if err != nil {
		return err
	}

	// Execute the PutItem operation
	_, err = d.ddbClient.PutItem(&dynamodb.PutItemInput{
		TableName:           aws.String("User"),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(userId)"),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return exception.NewUserAlreadyExistException(fmt.Sprintf("User with userID %s already exists", user.UserID), err)
			} else {
				return awsErr
			}
		}
		return err
	}

	return nil
}

func userMarshalMap(user model.User) (map[string]*dynamodb.AttributeValue, error) {
	// Marshal the user object into a DynamoDB attribute value map
	av, err := dynamodbattribute.MarshalMap(user)
	if err != nil {
		return av, err
	}

	// Replace the createdAt attribute value with the numeric representation
	av["createdAt"] = &dynamodb.AttributeValue{
		N: aws.String(strconv.FormatInt(user.CreatedAt.UnixNano(), 10)),
	}

	return av, nil
}
