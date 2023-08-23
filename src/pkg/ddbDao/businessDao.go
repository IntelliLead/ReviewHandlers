package ddbDao

import (
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
func (d *BusinessDao) CreateBusiness(Business model.Business) error {
    d.log.Debug("Putting Business in DDB if not exist: ", Business)

    av, err := BusinessMarshalMap(Business)
    if err != nil {
        return err
    }

    // Execute the PutItem operation
    d.log.Debug("Executing PutItem operation in DDB")

    _, err = d.client.PutItem(&dynamodb.PutItemInput{
        TableName:           aws.String(enum.TableBusiness.String()),
        Item:                av,
        ConditionExpression: aws.String(KeyNotExistsConditionExpression),
    })
    if err != nil {
        d.log.Debug("Error putting Business in DDB: ", err)

        if awsErr, ok := err.(awserr.Error); ok {
            if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
                return exception.NewBusinessAlreadyExistException(fmt.Sprintf("Business with BusinessID %s already exist", Business.BusinessId), err)
            } else {
                return awsErr
            }
        }
        return err
    }

    d.log.Debug("Successfully put new business in DDB: ", Business)

    return nil
}

// GetBusiness gets a Business with the given BusinessId from the Business table
// If the Business does not exist, returns nil, nil
func (d *BusinessDao) GetBusiness(BusinessId string) (*model.Business, error) {
    response, err := d.client.GetItem(&dynamodb.GetItemInput{
        TableName: aws.String(enum.TableBusiness.String()),
        Key:       model.BuildDdbBusinessKey(BusinessId),
    })
    if err != nil {
        d.log.Errorf("Unable to get item with BusinessId '%s' in GetBusiness: %v", BusinessId, err)

        switch err.(type) {
        case *dynamodb.ResourceNotFoundException:
            return nil, nil
        }

        d.log.Error("Unknown error in GetBusiness: ", err)
        return nil, exception.NewUnknownDDBException(fmt.Sprintf("GetBusiness failed for BusinessId '%s' with unknown error: ", BusinessId), err)
    }

    var business *model.Business
    err = dynamodbattribute.UnmarshalMap(response.Item, business)
    if err != nil {
        d.log.Errorf("Unable to unmarshal from DDB response '%s' to Business object in GetBusiness: %v",
            jsonUtil.AnyToJson(response.Item), err)
        return nil, err
    }

    return business, nil
}

func BusinessMarshalMap(Business model.Business) (map[string]*dynamodb.AttributeValue, error) {
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
func (d *BusinessDao) UpdateAttributes(BusinessId string, actions []dbModel.AttributeAction) (model.Business, error) {
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

        case enum.ActionAppend:
            updateBuilder = updateBuilder.Set(expression.Name(action.Name), expression.ListAppend(expression.Name(action.Name), expression.Value(action.Value)))
        }
    }

    expr, err := expression.NewBuilder().WithUpdate(updateBuilder).Build()
    if err != nil {
        d.log.Errorf("Unable to build expression for UpdateItem in UpdateAttributes: %v", err)
        return model.Business{}, err
    }

    allNewStr := dynamodb.ReturnValueAllNew
    // Execute the UpdateItem operation
    ddbInput := &dynamodb.UpdateItemInput{
        TableName:                 aws.String(enum.TableBusiness.String()),
        Key:                       model.BuildDdbBusinessKey(BusinessId),
        UpdateExpression:          expr.Update(),
        ExpressionAttributeNames:  expr.Names(),
        ExpressionAttributeValues: expr.Values(),
        ReturnValues:              &allNewStr,
    }
    d.log.Debugf("DDB UpdateItem input: %s", jsonUtil.AnyToJson(ddbInput))
    response, err := d.client.UpdateItem(ddbInput)
    if err != nil {
        d.log.Errorf("DDB UpdateItem failed in UpdateAttributes with DDB input '%s': %v", jsonUtil.AnyToJson(ddbInput), err)
        return model.Business{}, err
    }

    var Business model.Business
    err = dynamodbattribute.UnmarshalMap(response.Attributes, &Business)
    if err != nil {
        d.log.Errorf("Unable to unmarshal from DDB response '%s' to Business object in UpdateAttributes: %v",
            jsonUtil.AnyToJson(response.Attributes), err)
        return model.Business{}, err
    }

    return Business, nil
}
