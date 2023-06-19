package ddbDao

import (
    "fmt"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/dbModel"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/ddbDao/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/exception"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/jsonUtil"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model"
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
    "github.com/aws/aws-sdk-go/service/dynamodb/expression"
    "go.uber.org/zap"
    "time"
)

type ReviewDao struct {
    client *dynamodb.DynamoDB
    log    *zap.SugaredLogger
}

func NewReviewDao(client *dynamodb.DynamoDB, logger *zap.SugaredLogger) *ReviewDao {
    return &ReviewDao{
        client: client,
        log:    logger,
    }
}

func (d *ReviewDao) GetNextReviewID(userId string) (_type.ReviewId, error) {
    // Define the expression to retrieve the largest ReviewId for the given UserId
    expr, err := expression.NewBuilder().
        WithKeyCondition(expression.Key("userId").Equal(expression.Value(userId))).Build()
    if err != nil {
        d.log.Error("Unable to produce key condition expression for GetNextReviewID with userId %s: ", userId, err)
        return "", err
    }

    result, err := d.client.Query(&dynamodb.QueryInput{
        TableName:                 aws.String(enum.TableReview.String()),
        KeyConditionExpression:    expr.KeyCondition(),
        ExpressionAttributeNames:  expr.Names(),
        ExpressionAttributeValues: expr.Values(),
        ScanIndexForward:          aws.Bool(false), // get largest
        Limit:                     aws.Int64(1),
    })

    if err != nil {
        d.log.Error("Unable to execute query in GetNextReviewID with userId %s: ", userId, err)
        return "", err
    }

    // If there are no existing reviews, start with ReviewId 1
    if len(result.Items) == 0 {
        return _type.NewReviewId("0"), nil
    }

    // Extract the largest ReviewId
    var review model.Review
    err = dynamodbattribute.UnmarshalMap(result.Items[0], &review)
    if err != nil {
        d.log.Error("Unable to unmarshal the first query result in GetNextReviewID with query response %s: ", result.Items[0], err)
        return "", err
    }

    return (*review.ReviewId).GetNext(), nil
}

// CreateReview creates a new review in DynamoDB
func (d *ReviewDao) CreateReview(review model.Review) error {
    err := model.ValidateReview(&review)
    if err != nil {
        d.log.Error("CreateReview failed due to invalid review: ", jsonUtil.AnyToJson(review))
        return err
    }

    uniqueVendorReviewID := dbModel.NewUniqueVendorReviewIdRecord(review)

    av, err := dynamodbattribute.MarshalMap(review)
    if err != nil {
        return err
    }

    uniqueAv, err := dynamodbattribute.MarshalMap(uniqueVendorReviewID)
    if err != nil {
        return err
    }

    _, err = d.client.TransactWriteItems(&dynamodb.TransactWriteItemsInput{
        TransactItems: []*dynamodb.TransactWriteItem{
            {
                Put: &dynamodb.Put{
                    TableName:           aws.String(enum.TableReview.String()),
                    Item:                av,
                    ConditionExpression: aws.String(KeyNotExistsConditionExpression),
                },
            },
            {
                Put: &dynamodb.Put{
                    TableName:           aws.String(enum.TableReview.String()),
                    Item:                uniqueAv,
                    ConditionExpression: aws.String(KeyNotExistsConditionExpression),
                },
            },
        },
    })
    if err != nil {
        switch t := err.(type) {
        case *dynamodb.TransactionCanceledException:
            failedRequests := t.CancellationReasons
            // assert length should be 2
            if len(failedRequests) != 2 {
                return exception.NewUnknownTransactionCanceledException("Transaction failed in CreateReview for unknown reasons - unexpected CancellationReasons length: ", err)
            }

            if *(failedRequests[0].Code) == dynamodb.BatchStatementErrorCodeEnumConditionalCheckFailed {
                return exception.NewReviewAlreadyExistException(fmt.Sprintf("Review with reviewID %s already exists", review.ReviewId.String()), err)
            }

            if *(failedRequests[1].Code) == dynamodb.BatchStatementErrorCodeEnumConditionalCheckFailed {
                return exception.NewVendorReviewIdAlreadyExistException(fmt.Sprintf("UniqueVendorReviewId with vendorReviewID %s already exists", review.VendorReviewId), err)
            }

            return exception.NewUnknownTransactionCanceledException("Transaction failed in CreateReview for unknown reasons: ", err)
        default:
            d.log.Error("CreateReview TransactWriteItems failed for unknown reason: ", jsonUtil.AnyToJson(err))
            return exception.NewUnknownDDBException("CreateReview TransactWriteItems failed for unknown reason: ", err)
        }
    }

    return nil
}

type UpdateReviewInput struct {
    UserId      string         `dynamodbav:"userId"`
    ReviewId    _type.ReviewId `dynamodbav:"uniqueId"`
    LastUpdated time.Time      `dynamodbav:"lastUpdated"`
    LastReplied time.Time      `dynamodbav:"lastReplied"`
    Reply       string         `dynamodbav:"reply"`
}

func (d *ReviewDao) UpdateReview(input UpdateReviewInput) error {
    av, err := dynamodbattribute.MarshalMap(input)
    if err != nil {
        return err
    }

    // [INT-44] BUG observation
    d.log.Debug("UpdateReview ExpressionAttributeValues: ", jsonUtil.AnyToJson(input))

    // Create the key for the UpdateItem request
    key, err := dynamodbattribute.MarshalMap(map[string]interface{}{
        "userId":   input.UserId,
        "uniqueId": input.ReviewId,
    })
    if err != nil {
        return err
    }

    // Execute the UpdateItem operation
    ddbInput := &dynamodb.UpdateItemInput{
        TableName:                 aws.String(enum.TableReview.String()),
        Key:                       key,
        UpdateExpression:          aws.String("SET #lu = :lu, #lr = :lr, #rep = :rep"),
        ExpressionAttributeNames:  map[string]*string{"#lu": aws.String("lastUpdated"), "#lr": aws.String("lastReplied"), "#rep": aws.String("reply")},
        ExpressionAttributeValues: av,
    }
    _, err = d.client.UpdateItem(ddbInput)
    if err != nil {
        d.log.Errorf("UpdateItem failed in UpdateReview with input '%s': %v", jsonUtil.AnyToJson(ddbInput), err)
        return err
    }

    return nil
}

func (d *ReviewDao) GetReview(userId string, reviewId _type.ReviewId) (model.Review, error) {
    // Create the key for the GetItem request
    key, err := dynamodbattribute.MarshalMap(map[string]interface{}{
        "userId":   userId,
        "uniqueId": reviewId,
    })
    if err != nil {
        return model.Review{}, err
    }

    result, err := d.client.GetItem(&dynamodb.GetItemInput{
        TableName: aws.String(enum.TableReview.String()),
        Key:       key,
    })
    if err != nil {
        d.log.Debugf("GetReview failed for userId %s reviewId %s: %s", userId, reviewId, jsonUtil.AnyToJson(err))

        return model.Review{}, exception.NewUnknownDDBException(fmt.Sprintf("GetReview failed for userId %s reviewId %s with unknown error: ", userId, reviewId), err)
    }

    // Check if the item was found
    if result.Item == nil {
        return model.Review{}, exception.NewReviewDoesNotExistException(fmt.Sprintf("Review with userId '%s' reviewID '%s' not found", userId, reviewId))
    }

    // Unmarshal the item into a review object
    review := &model.Review{}
    err = dynamodbattribute.UnmarshalMap(result.Item, review)
    if err != nil {
        return model.Review{}, fmt.Errorf("failed to unmarshal Review, %v", err)
    }

    err = model.ValidateReview(review)
    if err != nil {
        return model.Review{}, fmt.Errorf("invalid review fetched: %v", err)
    }

    return *review, nil
}
