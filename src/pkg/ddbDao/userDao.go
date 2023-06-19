package ddbDao

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
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
                return exception.NewUserAlreadyExistException(fmt.Sprintf("User with userID %s already exists", user.UserId), err)
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
    _, err := d.GetUser(userId)
    if err != nil {
        if _, ok := err.(*exception.UserDoesNotExistException); ok {
            return false, nil
        }
        return false, err
    }

    return true, nil
}

// GetUser gets a user with the given userId from the User table
// error handling
// 1. user does not exist UserDoesNotExistException
// 2. aws error
func (d *UserDao) GetUser(userId string) (model.User, error) {
    expr, err := expression.NewBuilder().WithKeyCondition(expression.Key("userId").Equal(expression.Value(userId))).Build()
    if err != nil {
        d.log.Errorf("Unable to produce key condition expression in GetUser with userId %s: %v", userId, err)
        return model.User{}, err
    }

    response, err := d.client.GetItem(&dynamodb.GetItemInput{
        TableName:                aws.String(enum.TableUser.String()),
        Key:                      expr.Values(),
        ExpressionAttributeNames: expr.Names(),
    })
    if err != nil {
        d.log.Errorf("Unable to get item with userId '%s' in GetUser: %v", userId, err)

        switch err.(type) {
        case *dynamodb.ResourceNotFoundException:
            return model.User{}, exception.NewUserDoesNotExistExceptionWithErr(fmt.Sprintf("User with userId %s does not exist", userId), err)
        default:
            d.log.Error("Unknown error in GetUser: ", err)
            return model.User{}, err
        }
    }

    var user model.User
    err = dynamodbattribute.UnmarshalMap(response.Item, &user)
    if err != nil {
        d.log.Errorf("Unable to unmarshal from DDB response '%s' to User object in GetUser: %v",
            jsonUtil.AnyToJson(response.Item), err)
        return model.User{}, err
    }

    return user, nil
}

type updateQuickReplyMessageAttributeValue struct {
    QuickReplyMessage string `dynamodbav:"quickReplyMessage"`
}

func (d *UserDao) UpdateQuickReplyMessage(userId string, quickReplyMessage string) (model.User, error) {
    av, err := dynamodbattribute.MarshalMap(updateQuickReplyMessageAttributeValue{
        QuickReplyMessage: quickReplyMessage,
    })
    if err != nil {
        d.log.Errorf("Unable to marshal attribute value '%s' in UpdateQuickReplyMessage: %v", quickReplyMessage, err)
        return model.User{}, err
    }

    key, err := dynamodbattribute.MarshalMap(map[string]interface{}{
        "userId":   userId,
        "uniqueId": util.DefaultUniqueId,
    })
    if err != nil {
        return model.User{}, err
    }

    allNewStr := dynamodb.ReturnValueAllNew
    // Execute the UpdateItem operation
    ddbInput := &dynamodb.UpdateItemInput{
        TableName:                 aws.String(enum.TableUser.String()),
        Key:                       key,
        UpdateExpression:          aws.String("SET #qrm = :qrm"),
        ExpressionAttributeNames:  map[string]*string{"#qrm": aws.String("quickReplyMessage")},
        ExpressionAttributeValues: av,
        ReturnValues:              &allNewStr,
    }
    response, err := d.client.UpdateItem(ddbInput)
    if err != nil {
        d.log.Errorf("DDB UpdateItem failed in UpdateQuickReplyMessage with input '%s': %v", jsonUtil.AnyToJson(ddbInput), err)
        return model.User{}, err
    }

    var user model.User
    err = dynamodbattribute.UnmarshalMap(response.Attributes, &user)
    if err != nil {
        d.log.Errorf("Unable to unmarshal from DDB response '%s' to User object in GetUser: %v",
            jsonUtil.AnyToJson(response.Attributes), err)
        return model.User{}, err
    }

    return user, nil
}

func (d *UserDao) DeleteQuickReplyMessage(userId string) (model.User, error) {
    key, err := dynamodbattribute.MarshalMap(map[string]interface{}{
        "userId":   userId,
        "uniqueId": util.DefaultUniqueId,
    })
    if err != nil {
        return model.User{}, err
    }

    allNewStr := dynamodb.ReturnValueAllNew
    // Execute the UpdateItem operation
    ddbInput := &dynamodb.UpdateItemInput{
        TableName:                aws.String(enum.TableUser.String()),
        Key:                      key,
        UpdateExpression:         aws.String("REMOVE #qrm = :qrm"),
        ExpressionAttributeNames: map[string]*string{"#qrm": aws.String("quickReplyMessage")},
        ReturnValues:             &allNewStr,
    }
    response, err := d.client.UpdateItem(ddbInput)
    if err != nil {
        d.log.Errorf("DDB UpdateItem failed in UpdateQuickReplyMessage with input '%s': %v", jsonUtil.AnyToJson(ddbInput), err)
        return model.User{}, err
    }

    var user model.User
    err = dynamodbattribute.UnmarshalMap(response.Attributes, &user)
    if err != nil {
        d.log.Errorf("Unable to unmarshal from DDB response '%s' to User object in GetUser: %v",
            jsonUtil.AnyToJson(response.Attributes), err)
        return model.User{}, err
    }

    // DEBUG assert that quickReplyMessage is null
    if user.QuickReplyMessage != nil {
        d.log.Fatal("quickReplyMessage is not null after deletion in DeleteQuickReplyMessage")
    }

    return user, nil
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

    // DEBUG
    // logger.NewLogger().Debug("userMarshalMap after uniqueId add: ", av)

    return av, nil
}
