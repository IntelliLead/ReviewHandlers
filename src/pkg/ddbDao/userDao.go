package ddbDao

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/awserr"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
    "github.com/aws/aws-sdk-go/service/dynamodb/expression"
    "go.uber.org/zap"
    "strconv"
)

type UserDao struct {
    client *dynamodb.DynamoDB
    log    *zap.SugaredLogger
}

func NewUserDao(client *dynamodb.DynamoDB, logger *zap.SugaredLogger) *UserDao {
    return &UserDao{
        client: client,
        log:    logger,
    }
}

// CreateUser creates a new user in the User table
// error handling
// 1. user already exist UserAlreadyExistException
// 2. aws error
func (d *UserDao) CreateUser(user model.User) error {
    d.log.Debug("Putting user in DDB if not exist: ", user)

    av, err := userMarshalMap(user)
    if err != nil {
        return err
    }

    // Execute the PutItem operation
    d.log.Debug("Executing PutItem operation in DDB")

    _, err = d.client.PutItem(&dynamodb.PutItemInput{
        TableName:           aws.String(enum.TableUser.String()),
        Item:                av,
        ConditionExpression: aws.String(KeyNotExistsConditionExpression),
    })
    if err != nil {
        d.log.Debug("Error putting user in DDB: ", err)

        if awsErr, ok := err.(awserr.Error); ok {
            if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
                return exception.NewUserAlreadyExistException(fmt.Sprintf("User with userID %s already exists", user.UserID), err)
            } else {
                return awsErr
            }
        }
        return err
    }

    d.log.Debug("Successfully put user in DDB: ", user)

    return nil
}

// IsUserExist checks if a user with the given userId exists in the User table
func (d *UserDao) IsUserExist(userId string) (bool, error) {
    // Define the key condition expression for the query
    expr, err := expression.NewBuilder().WithKeyCondition(expression.Key("userId").Equal(expression.Value(userId))).Build()
    if err != nil {
        d.log.Errorf("Unable to produce key condition expression in IsUserExist with userId %s: %v", userId, err)
        return false, err
    }

    // TODO: [INT-31] use GetItem instead to improve performance and reduce cost
    // Execute the query
    result, err := d.client.Query(&dynamodb.QueryInput{
        TableName:                 aws.String(enum.TableUser.String()),
        KeyConditionExpression:    expr.KeyCondition(),
        ExpressionAttributeNames:  expr.Names(),
        ExpressionAttributeValues: expr.Values(),
        Limit:                     aws.Int64(1),
    })
    if err != nil {
        d.log.Error("Unable to query with userId %s: ", userId, err)
        return false, err
    }

    // Check if any items were returned
    if len(result.Items) > 0 {
        return true, nil
    }

    return false, nil
}

func userMarshalMap(user model.User) (map[string]*dynamodb.AttributeValue, error) {
    // Marshal the user object into a DynamoDB attribute value map
    av, err := dynamodbattribute.MarshalMap(user)
    if err != nil {
        return av, err
    }

    // Replace attribute values with their numeric representation
    av["createdAt"] = &dynamodb.AttributeValue{
        N: aws.String(strconv.FormatInt(user.CreatedAt.UnixNano(), 10)),
    }
    av["lastUpdated"] = &dynamodb.AttributeValue{
        N: aws.String(strconv.FormatInt(user.LastUpdated.UnixNano(), 10)),
    }
    if user.ExpireAt != nil {
        av["expireAt"] = &dynamodb.AttributeValue{
            N: aws.String(strconv.FormatInt(user.ExpireAt.UnixNano(), 10)),
        }
    }

    // add sort key
    // (sort key appears already added somehow, just mistakenly as 'N' type)
    av["uniqueId"] = &dynamodb.AttributeValue{
        S: aws.String("#"),
    }

    // logger.NewLogger().Debug("userMarshalMap after uniqueId add: ", av)

    return av, nil
}
