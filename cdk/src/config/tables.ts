import {
    Attribute,
    AttributeType,
    BillingMode,
    GlobalSecondaryIndexProps,
    LocalSecondaryIndexProps,
    ProjectionType,
} from 'aws-cdk-lib/aws-dynamodb';

/**
 * Interface to guide creation of DynamoDb tables.
 */
export interface DynamoDbTableAttribute {
    readonly tableName: TableName;
    readonly partitionKey: Attribute;
    readonly sortKey?: Attribute;
    readonly globalSecondaryIndexes?: GlobalSecondaryIndexProps[];
    readonly localSecondaryIndexes?: LocalSecondaryIndexProps[];
    readonly billingMode: BillingMode;
}

export enum TableName {
    REVIEW = 'Review',
    USER = 'User',
    BUSINESS = 'Business,
}

const reviewTable: DynamoDbTableAttribute = {
    tableName: TableName.REVIEW,
    partitionKey: {
        name: 'userId',
        type: AttributeType.STRING,
    },
    sortKey: {
        name: 'uniqueId',
        type: AttributeType.STRING,
    },
    localSecondaryIndexes: [
        {
            indexName: 'createdAt-lsi',
            projectionType: ProjectionType.ALL,
            sortKey: {
                name: 'createdAt',
                type: AttributeType.NUMBER,
            },
        },
        {
            indexName: 'lastReplied-lsi',
            projectionType: ProjectionType.ALL,
            sortKey: {
                name: 'lastReplied',
                type: AttributeType.NUMBER,
            },
        },
        {
            indexName: 'lastUpdated-lsi',
            projectionType: ProjectionType.ALL,
            sortKey: {
                name: 'lastUpdated',
                type: AttributeType.NUMBER,
            },
        },
        {
            indexName: 'numberRating-lsi',
            projectionType: ProjectionType.ALL,
            sortKey: {
                name: 'numberRating',
                type: AttributeType.NUMBER,
            },
        },
        {
            indexName: 'reviewLastUpdated-lsi',
            projectionType: ProjectionType.ALL,
            sortKey: {
                name: 'reviewLastUpdated',
                type: AttributeType.NUMBER,
            },
        },
    ],
    billingMode: BillingMode.PAY_PER_REQUEST,
};

const userTable: DynamoDbTableAttribute = {
    tableName: TableName.USER,
    partitionKey: {
        name: 'userId',
        type: AttributeType.STRING,
    },
    sortKey: {
        name: 'uniqueId',
        type: AttributeType.STRING,
    },
    localSecondaryIndexes: [
        {
            indexName: 'createdAt-lsi',
            projectionType: ProjectionType.ALL,
            sortKey: {
                name: 'createdAt',
                type: AttributeType.NUMBER,
            },
        },
        {
            indexName: 'lastUpdated-lsi',
            projectionType: ProjectionType.ALL,
            sortKey: {
                name: 'lastUpdated',
                type: AttributeType.NUMBER,
            },
        },
    ],
    billingMode: BillingMode.PAY_PER_REQUEST,
};

const businessTable: DynamoDbTableAttribute = {
    tableName: TableName.BUSINESS,
    partitionKey: {
        name: 'businessId',
        type: AttributeType.STRING,
    },
    sortKey: {
        name: 'uniqueId',
        type: AttributeType.STRING,
    },
    localSecondaryIndexes: [
        {
            indexName: 'createdAt-lsi',
            projectionType: ProjectionType.ALL,
            sortKey: {
                name: 'createdAt',
                type: AttributeType.NUMBER,
            },
        },
        {
            indexName: 'lastUpdated-lsi',
            projectionType: ProjectionType.ALL,
            sortKey: {
                name: 'lastUpdated',
                type: AttributeType.NUMBER,
            },
        },
    ],
    billingMode: BillingMode.PAY_PER_REQUEST,
};
export const Tables: DynamoDbTableAttribute[] = [reviewTable, userTable, businessTable];
