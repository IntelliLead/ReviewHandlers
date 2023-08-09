package ddbDao

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/awserr"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
    "github.com/aws/aws-sdk-go/service/dynamodb/expression"
    "go.uber.org/zap"
    "strings"
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
func (d *UserDao) IsUserExist(userId string) (bool, model.User, error) {
    user, err := d.GetUser(userId)
    if err != nil {
        if _, ok := err.(*exception.UserDoesNotExistException); ok {
            return false, model.User{}, nil
        }
        return false, model.User{}, err
    }

    return true, user, nil
}

// GetUser gets a user with the given userId from the User table
// error handling
// 1. user does not exist UserDoesNotExistException
// 2. aws error
func (d *UserDao) GetUser(userId string) (model.User, error) {
    response, err := d.client.GetItem(&dynamodb.GetItemInput{
        TableName: aws.String(enum.TableUser.String()),
        Key:       model.BuildDdbUserKey(userId),
    })
    if err != nil {
        d.log.Errorf("Unable to get item with userId '%s' in GetUser: %v", userId, err)

        switch err.(type) {
        case *dynamodb.ResourceNotFoundException:
            return model.User{}, exception.NewUserDoesNotExistExceptionWithErr(fmt.Sprintf("User with userId %s does not exist", userId), err)
        default:
            d.log.Error("Unknown error in GetUser: ", err)
        }
        return model.User{}, exception.NewUnknownDDBException(fmt.Sprintf("GetUser failed for userId '%s' with unknown error: ", userId), err)
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

func userMarshalMap(user model.User) (map[string]*dynamodb.AttributeValue, error) {
    // Marshal the user object into a DynamoDB attribute value map
    av, err := dynamodbattribute.MarshalMap(user)
    if err != nil {
        return av, err
    }

    // add sort key
    // (sort key appears already added somehow, just mistakenly as 'N' type)
    av["uniqueId"] = &dynamodb.AttributeValue{
        S: aws.String("#"),
    }

    return av, nil
}

type AttributeAction struct {
    Action enum.Action // "update" or "delete"
    Name   string      // Name of the attribute
    Value  interface{} // Value to set (for updates only)
}

// UpdateAttributes updates and deletes attributes of a user with the given userId.
// Note that deleting required fields may break the data model.
// For example:
// user, err = userDao.UpdateAttributes(userId, []AttributeAction{
//     {
//         Action: enum.ActionDelete,
//         Name:   "businessDescription",
//     },
//     {
//         Action: enum.ActionUpdate,
//         Name:   "arrayField",
//         Value:  []string{"keyword1", "keyword2"},
//     }
// }    )
func (d *UserDao) UpdateAttributes(userId string, actions []AttributeAction) (model.User, error) {
    err := validateUniqueAttributeNames(actions)
    if err != nil {
        return model.User{}, err
    }

    var updateBuilder expression.UpdateBuilder
    for _, action := range actions {
        attribute := strings.ToLower(string(action.Name[0])) + action.Name[1:] // for safety in case of typo

        switch action.Action {
        case enum.ActionDelete:
            updateBuilder = updateBuilder.Remove(expression.Name(action.Name))

        case enum.ActionUpdate:
            updateBuilder = updateBuilder.Set(expression.Name(attribute), expression.Value(action.Value))
        }
    }

    expr, err := expression.NewBuilder().WithUpdate(updateBuilder).Build()
    if err != nil {
        d.log.Errorf("Unable to build expression for UpdateItem in UpdateAttributes: %v", err)
        return model.User{}, err
    }

    allNewStr := dynamodb.ReturnValueAllNew
    // Execute the UpdateItem operation
    ddbInput := &dynamodb.UpdateItemInput{
        TableName:                 aws.String(enum.TableUser.String()),
        Key:                       model.BuildDdbUserKey(userId),
        UpdateExpression:          expr.Update(),
        ExpressionAttributeNames:  expr.Names(),
        ExpressionAttributeValues: expr.Values(),
        ReturnValues:              &allNewStr,
    }
    d.log.Debugf("DDB UpdateItem input: %s", jsonUtil.AnyToJson(ddbInput))
    response, err := d.client.UpdateItem(ddbInput)
    if err != nil {
        d.log.Errorf("DDB UpdateItem failed in UpdateAttributes with DDB input '%s': %v", jsonUtil.AnyToJson(ddbInput), err)
        return model.User{}, err
    }

    var user model.User
    err = dynamodbattribute.UnmarshalMap(response.Attributes, &user)
    if err != nil {
        d.log.Errorf("Unable to unmarshal from DDB response '%s' to User object in UpdateAttributes: %v",
            jsonUtil.AnyToJson(response.Attributes), err)
        return model.User{}, err
    }

    return user, nil
}

func validateUniqueAttributeNames(actions []AttributeAction) error {
    if len(actions) == 0 {
        return errors.New(fmt.Sprintf("No actions provided to UpdateAttributes"))
    }

    uniqueNames := make(map[string]bool)
    for _, action := range actions {
        if _, ok := uniqueNames[action.Name]; ok {
            return errors.New(fmt.Sprintf("Duplicate attribute name '%s' in UpdateAttributes", action.Name))
        }
        uniqueNames[action.Name] = true
    }
    return nil
}
