package ddbDao

import (
    "errors"
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/dbModel"
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
)

type BusinessDao struct {
    client *dynamodb.DynamoDB
    log    *zap.SugaredLogger
}

func NewBusinessDao(client *dynamodb.DynamoDB, logger *zap.SugaredLogger) *BusinessDao {
    return &BusinessDao{
        client: client,
        log:    logger,
    }
}

// CreateBusiness creates a new Business in the Business table
// error handling
// 1. Business already exist BusinessAlreadyExistException
// 2. aws error
func (b *BusinessDao) CreateBusiness(Business model.Business) error {
    av, err := b.marshalMap(Business)
    if err != nil {
        return err
    }

    putItemInput := dynamodb.PutItemInput{
        TableName:           aws.String(enum.TableBusiness.String()),
        Item:                av,
        ConditionExpression: aws.String(KeyNotExistsConditionExpression),
    }
    _, err = b.client.PutItem(&putItemInput)
    if err != nil {
        b.log.Debugf("Error putting Business %s in DDB: %v", jsonUtil.AnyToJson(putItemInput), err)

        if awsErr, ok := err.(awserr.Error); ok {
            if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
                return exception.NewBusinessAlreadyExistException(fmt.Sprintf("Business with BusinessID %s already exist", Business.BusinessId), err)
            } else {
                return awsErr
            }
        }
        return err
    }

    return nil
}

// GetBusiness gets a Business with the given BusinessId from the Business table
// If the Business does not exist, returns nil, nil
func (b *BusinessDao) GetBusiness(BusinessId string) (*model.Business, error) {
    response, err := b.client.GetItem(&dynamodb.GetItemInput{
        TableName: aws.String(enum.TableBusiness.String()),
        Key:       model.BuildDdbBusinessKey(BusinessId),
    })
    if err != nil {
        b.log.Errorf("Unable to get item with BusinessId '%s' in GetBusiness: %v", BusinessId, err)
        return nil, exception.NewUnknownDDBException(fmt.Sprintf("GetBusiness failed for BusinessId '%s' with unknown error: ", BusinessId), err)
    }

    if response.Item == nil {
        return nil, nil
    }

    var business model.Business
    err = dynamodbattribute.UnmarshalMap(response.Item, &business)
    if err != nil {
        b.log.Errorf("Unable to unmarshal from DDB response '%s' to Business object in GetBusiness: %v",
            jsonUtil.AnyToJson(response.Item), err)
        return nil, err
    }

    return &business, nil
}

func (b *BusinessDao) marshalMap(Business model.Business) (map[string]*dynamodb.AttributeValue, error) {
    // Marshal the Business object into a DynamoDB attribute value map
    av, err := dynamodbattribute.MarshalMap(Business)
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

// UpdateAttributes updates and deletes attributes of a Business with the given BusinessId.
// Note that deleting required fields may break the data model.
// For example:
// Business, err = BusinessDao.UpdateAttributes(BusinessId, []AttributeAction{
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
func (b *BusinessDao) UpdateAttributes(BusinessId string, actions []dbModel.AttributeAction) (model.Business, error) {
    err := dbModel.ValidateUniqueAttributeNames(actions)
    if err != nil {
        return model.Business{}, err
    }

    var updateBuilder expression.UpdateBuilder
    for _, action := range actions {
        switch action.Action {
        case enum.ActionRemove:
            updateBuilder = updateBuilder.Remove(expression.Name(action.Name))

        case enum.ActionUpdate:
            updateBuilder = updateBuilder.Set(expression.Name(action.Name), expression.Value(action.Value))

        case enum.ActionAppendStringSet:
            addSet := (&dynamodb.AttributeValue{}).SetSS(aws.StringSlice(action.Value.([]string)))
            updateBuilder = updateBuilder.Add(expression.Name(action.Name), expression.Value(addSet))

        default:
            b.log.Errorf("Unsupported action '%s' in businessDao.UpdateAttributes", action.Action)
            return model.Business{}, errors.New(fmt.Sprintf("Unsupported action '%s' in userDao.UpdateAttributes", action.Action))
        }
    }

    expr, err := expression.NewBuilder().WithUpdate(updateBuilder).Build()
    if err != nil {
        b.log.Errorf("Unable to build expression for UpdateItem in UpdateAttributes: %v", err)
        return model.Business{}, err
    }

    allNewStr := dynamodb.ReturnValueAllNew
    ddbInput := &dynamodb.UpdateItemInput{
        TableName:                 aws.String(enum.TableBusiness.String()),
        Key:                       model.BuildDdbBusinessKey(BusinessId),
        UpdateExpression:          expr.Update(),
        ExpressionAttributeNames:  expr.Names(),
        ExpressionAttributeValues: expr.Values(),
        ReturnValues:              &allNewStr,
    }
    response, err := b.client.UpdateItem(ddbInput)
    if err != nil {
        b.log.Errorf("DDB UpdateItem failed in UpdateAttributes with DDB input '%s': %v", jsonUtil.AnyToJson(ddbInput), err)
        return model.Business{}, err
    }

    var Business model.Business
    err = dynamodbattribute.UnmarshalMap(response.Attributes, &Business)
    if err != nil {
        b.log.Errorf("Unable to unmarshal from DDB response '%s' to Business object in UpdateAttributes: %v",
            jsonUtil.AnyToJson(response.Attributes), err)
        return model.Business{}, err
    }

    return Business, nil
}
