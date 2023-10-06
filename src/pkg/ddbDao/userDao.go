package ddbDao

import (
    "context"
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/dbModel"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "go.uber.org/zap"
    "strings"
    "time"
)

type UserDao struct {
    client *dynamodb.Client
    log    *zap.SugaredLogger
}

func NewUserDao(client *dynamodb.Client, logger *zap.SugaredLogger) *UserDao {
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
    av, err := userMarshalMap(user)
    if err != nil {
        return err
    }

    _, err = d.client.PutItem(context.TODO(), &dynamodb.PutItemInput{
        TableName:           aws.String(enum.TableUser.String()),
        Item:                av,
        ConditionExpression: aws.String(KeyNotExistsConditionExpression),
    })
    if err != nil {
        d.log.Debug("Error putting user in DDB: ", err)

        var conditionalCheckFailedException *types.ConditionalCheckFailedException
        switch {
        case errors.As(err, &conditionalCheckFailedException):
            return exception.NewUserAlreadyExistException(fmt.Sprintf("User with userID %s already exists", user.UserId), err)
        default:
            return err
        }
    }

    return nil
}

// GetUser gets a user with the given userId from the User table
func (d *UserDao) GetUser(userId string) (*model.User, error) {
    response, err := d.client.GetItem(context.TODO(), &dynamodb.GetItemInput{
        TableName: aws.String(enum.TableUser.String()),
        Key:       model.BuildDdbUserKey(userId),
    })
    if err != nil {
        d.log.Errorf("Unable to get item with userId '%s' in GetUser: %v", userId, err)
        return nil, exception.NewUnknownDDBException(fmt.Sprintf("GetUser failed for userId '%s' with unknown error: ", userId), err)
    }

    if response.Item == nil {
        return nil, nil
    }

    var user model.User
    err = attributevalue.UnmarshalMap(response.Item, &user)
    if err != nil {
        d.log.Errorf("Unable to unmarshal from DDB response '%s' to User object in GetUser: %v",
            jsonUtil.AnyToJson(response.Item), err)
        return nil, err
    }
    return &user, nil
}

func userMarshalMap(user model.User) (map[string]types.AttributeValue, error) {
    // Marshal the user object into a DynamoDB attribute value map
    av, err := attributevalue.MarshalMap(user)
    if err != nil {
        return av, err
    }

    // add sort key
    // (sort key appears already added somehow, just mistakenly as 'N' type)
    av["uniqueId"] = &types.AttributeValueMemberS{Value: "#"}

    return av, nil
}

// UpdateAttributes updates and deletes attributes of a user with the given userId.
// Note that deleting required fields may break the data model.
// For example:
// user, err = userDao.UpdateAttributes(userId, []AttributeAction{
//     {
//         Action: enum.ActionRemove,
//         Name:   "businessDescription",
//     },
//     {
//         Action: enum.ActionUpdate,
//         Name:   "arrayField",
//         Value:  []string{"keyword1", "keyword2"},
//     }
// }    )
func (d *UserDao) UpdateAttributes(userId string, actions []dbModel.AttributeAction) (model.User, error) {
    err := dbModel.ValidateUniqueAttributeNames(actions)
    if err != nil {
        return model.User{}, err
    }

    var updateBuilder expression.UpdateBuilder
    for _, action := range actions {
        attribute := strings.ToLower(string(action.Name[0])) + action.Name[1:] // for safety in case of typo

        switch action.Action {
        case enum.ActionRemove:
            updateBuilder = updateBuilder.Remove(expression.Name(action.Name))

        case enum.ActionUpdate:
            updateBuilder = updateBuilder.Set(expression.Name(attribute), expression.Value(action.Value))

        case enum.ActionAppendStringSet:
            addSet := &types.AttributeValueMemberSS{Value: action.Value.([]string)}
            updateBuilder = updateBuilder.Add(expression.Name(action.Name), expression.Value(addSet))

        default:
            d.log.Errorf("Unsupported action '%s' in userDao.UpdateAttributes", action.Action)
            return model.User{}, errors.New(fmt.Sprintf("Unsupported action '%s' in userDao.UpdateAttributes", action.Action))
        }
    }

    // update timestamp (epoch seconds)
    // TODO: [INT-90] use ms instead of s
    updateBuilder = updateBuilder.Set(expression.Name("lastUpdated"), expression.Value(time.Now().Unix()))

    expr, err := expression.NewBuilder().WithUpdate(updateBuilder).Build()
    if err != nil {
        d.log.Errorf("Unable to build expression for UpdateItem in UpdateAttributes: %v", err)
        return model.User{}, err
    }

    // Execute the UpdateItem operation
    ddbInput := &dynamodb.UpdateItemInput{
        TableName:                 aws.String(enum.TableUser.String()),
        Key:                       model.BuildDdbUserKey(userId),
        UpdateExpression:          expr.Update(),
        ExpressionAttributeNames:  expr.Names(),
        ExpressionAttributeValues: expr.Values(),
        ReturnValues:              types.ReturnValueAllNew,
    }
    response, err := d.client.UpdateItem(context.TODO(), ddbInput)
    if err != nil {
        d.log.Errorf("DDB UpdateItem failed in UpdateAttributes with DDB input '%s': %v", jsonUtil.AnyToJson(ddbInput), err)
        return model.User{}, err
    }

    var user model.User
    err = attributevalue.UnmarshalMap(response.Attributes, &user)
    if err != nil {
        d.log.Errorf("Unable to unmarshal from DDB response '%s' to User object in UpdateAttributes: %v",
            jsonUtil.AnyToJson(response.Attributes), err)
        return model.User{}, err
    }

    return user, nil
}
