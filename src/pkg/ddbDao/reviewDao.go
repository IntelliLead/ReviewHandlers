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
    _type "github.com/IntelliLead/ReviewHandlers/src/pkg/model/type"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/util"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "go.uber.org/zap"
    "time"
)

type ReviewDao struct {
    client *dynamodb.Client
    log    *zap.SugaredLogger
}

func NewReviewDao(client *dynamodb.Client, logger *zap.SugaredLogger) *ReviewDao {
    return &ReviewDao{
        client: client,
        log:    logger,
    }
}

func (d *ReviewDao) GetNextReviewID(businessId string) (_type.ReviewId, error) {
    // Define the expression to retrieve the largest ReviewId for the given BusinessId
    expr, err := expression.NewBuilder().
        WithKeyCondition(expression.Key(util.ReviewTablePartitionKey).Equal(expression.Value(businessId))).
        Build()
    if err != nil {
        d.log.Error("Unable to produce key condition expression for GetNextReviewID with businessId %s: ", businessId, err)
        return "", err
    }

    result, err := d.client.Query(context.TODO(), &dynamodb.QueryInput{
        TableName:                 aws.String(enum.TableReview.String()),
        IndexName:                 aws.String(ReviewIndexCreatedAtLsi.String()),
        KeyConditionExpression:    expr.KeyCondition(),
        ExpressionAttributeNames:  expr.Names(),
        ExpressionAttributeValues: expr.Values(),
        ScanIndexForward:          aws.Bool(false), // get largest
        Limit:                     aws.Int32(1),
    })
    if err != nil {
        d.log.Error("Unable to execute query in GetNextReviewID with businessId %s: ", businessId, err)
        return "", err
    }

    // If there are no existing reviews, start with ReviewId 0
    if len(result.Items) == 0 {
        return _type.NewReviewId("0"), nil
    }

    // Extract the current largest ReviewId
    var review model.Review
    err = attributevalue.UnmarshalMap(result.Items[0], &review)
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

    av, err := attributevalue.MarshalMap(review)
    if err != nil {
        return err
    }

    uniqueAv, err := attributevalue.MarshalMap(uniqueVendorReviewID)
    if err != nil {
        return err
    }

    _, err = d.client.TransactWriteItems(context.TODO(), &dynamodb.TransactWriteItemsInput{
        TransactItems: []types.TransactWriteItem{
            {
                Put: &types.Put{
                    TableName:           aws.String(enum.TableReview.String()),
                    Item:                av,
                    ConditionExpression: aws.String(KeyNotExistsConditionExpression),
                },
            },
            {
                Put: &types.Put{
                    TableName:           aws.String(enum.TableReview.String()),
                    Item:                uniqueAv,
                    ConditionExpression: aws.String(KeyNotExistsConditionExpression),
                },
            },
        },
    })
    if err != nil {
        var t *types.TransactionCanceledException
        switch {
        case errors.As(err, &t):
            failedRequests := t.CancellationReasons

            if *(failedRequests[0].Code) == string(types.BatchStatementErrorCodeEnumConditionalCheckFailed) {
                return exception.NewReviewAlreadyExistException(fmt.Sprintf("Review with reviewID %s already exists", review.ReviewId.String()), err)
            }

            if *(failedRequests[1].Code) == string(types.BatchStatementErrorCodeEnumConditionalCheckFailed) {
                return exception.NewVendorReviewIdAlreadyExistException(fmt.Sprintf("UniqueVendorReviewId with vendorReviewID %s already exists", review.VendorReviewId), err)
            }

            d.log.Debug("CreateReview TransactWriteItems failed for unknown reason: ", jsonUtil.AnyToJson(err))

            return err
        default:
            d.log.Error("CreateReview TransactWriteItems failed for unknown reason: ", jsonUtil.AnyToJson(err))
            return exception.NewUnknownDDBException("CreateReview TransactWriteItems failed for unknown reason: ", err)
        }
    }

    return nil
}

type UpdateReviewInput struct {
    BusinessId  string         `dynamodbav:"userId"`
    ReviewId    _type.ReviewId `dynamodbav:"uniqueId"`
    LastUpdated time.Time      `dynamodbav:"lastUpdated"` // unixtime does not work
    LastReplied time.Time      `dynamodbav:"lastReplied"` // unixtime does not work
    Reply       string         `dynamodbav:"reply"`
    RepliedBy   string         `dynamodbav:"repliedBy"` // userId
}

func (d *ReviewDao) UpdateReview(input UpdateReviewInput) error {
    lastUpdatedAv, err := attributevalue.UnixTime(input.LastUpdated).MarshalDynamoDBAttributeValue()
    if err != nil {
        d.log.Error("Unable to marshal lastUpdated in UpdateReview: ", err)
    }

    lastRepliedAv, err := attributevalue.UnixTime(input.LastReplied).MarshalDynamoDBAttributeValue()
    if err != nil {
        d.log.Error("Unable to marshal lastReplied in UpdateReview: ", err)
    }

    update := expression.Set(
        expression.Name("lastUpdated"),
        expression.Value(lastUpdatedAv),
    ).Set(
        expression.Name("lastReplied"),
        expression.Value(lastRepliedAv),
    ).Set(
        expression.Name("reply"),
        expression.Value(input.Reply),
    ).Set(
        expression.Name("replyBy"),
        expression.Value(input.RepliedBy),
    )
    expr, err := expression.NewBuilder().
        WithUpdate(update).
        Build()
    if err != nil {
        d.log.Errorf("Unable to build expression for UpdateItem in UpdateReview: %v", err)
        return err
    }

    // Create the key for the UpdateItem request
    key, err := attributevalue.MarshalMap(map[string]interface{}{
        util.ReviewTablePartitionKey: input.BusinessId,
        util.ReviewTableSortKey:      input.ReviewId,
    })
    if err != nil {
        return err
    }

    // Execute the UpdateItem operation
    ddbInput := &dynamodb.UpdateItemInput{
        TableName:                 aws.String(enum.TableReview.String()),
        Key:                       key,
        UpdateExpression:          expr.Update(),
        ExpressionAttributeNames:  expr.Names(),
        ExpressionAttributeValues: expr.Values(),
    }
    _, err = d.client.UpdateItem(context.TODO(), ddbInput)
    if err != nil {
        var resourceNotFoundException *types.ResourceNotFoundException
        switch {
        case errors.As(err, &resourceNotFoundException):
            return exception.NewReviewDoesNotExistExceptionWithErr(
                fmt.Sprintf("Review with businessId '%s' and reviewId '%s' does not exist", input.BusinessId, input.ReviewId), err)
        default:
            d.log.Errorf("Unknown DDB error in UpdateReview with input '%s': %v", jsonUtil.AnyToJson(ddbInput), err)
            return err
        }
    }

    // TODO: unmarshal to review and return when necessary

    return nil
}

func (d *ReviewDao) GetReview(businessId string, reviewId _type.ReviewId) (*model.Review, error) {
    // Create the key for the GetItem request
    key, err := attributevalue.MarshalMap(map[string]interface{}{
        util.ReviewTablePartitionKey: businessId,
        util.ReviewTableSortKey:      reviewId,
    })
    if err != nil {
        return nil, err
    }

    result, err := d.client.GetItem(context.TODO(), &dynamodb.GetItemInput{
        TableName: aws.String(enum.TableReview.String()),
        Key:       key,
    })
    if err != nil {
        d.log.Errorf("GetReview failed for businessId '%s' reviewId '%s': %v", businessId, reviewId, err)
        return nil, err
    }

    if result.Item == nil {
        return nil, nil
    }

    // Unmarshal the item into a review object
    review := &model.Review{}
    err = attributevalue.UnmarshalMap(result.Item, review)
    if err != nil {
        return nil, fmt.Errorf("failed to unmarshal Review, %v", err)
    }

    err = model.ValidateReview(review)
    if err != nil {
        return nil, fmt.Errorf("invalid review fetched: %v", err)
    }

    return review, nil
}
