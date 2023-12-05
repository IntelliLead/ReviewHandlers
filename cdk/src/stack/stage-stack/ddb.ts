import { StackCreationInfo } from 'common-cdk';
import { CfnOutput, Stack } from 'aws-cdk-lib';
import { Table } from 'aws-cdk-lib/aws-dynamodb';
import { IRole } from 'aws-cdk-lib/aws-iam';
import { Construct } from 'constructs';

import { DynamoDbTableAttribute, TableName, DdbTable } from '../../config/ddbTable';

export interface DdbStackProps {
    readonly stackCreationInfo: StackCreationInfo;
    readonly terminationProtection?: boolean;
}

export class DdbStack extends Stack {
    public static grantTable(table: Table, grantedRole: IRole): void {
        table.grantFullAccess(grantedRole);
    }

    public tableEntries: Map<TableName, Table> = new Map();

    constructor(scope: Construct, id: string, props: DdbStackProps) {
        super(scope, id, props);

        DdbTable.forEach((table) => {
            const ddb = this.createTable(table);
            this.tableEntries.set(table.tableName, ddb);

            new CfnOutput(this, `${table.tableName}TableArnCfnOutput`, {
                value: ddb.tableArn,
                description: `DynamoDB table arn for ${table.tableName}`,
                exportName: `${table.tableName}TableArn`,
            });
        });
    }

    private createTable(definition: DynamoDbTableAttribute): Table {
        const table = new Table(this, `${definition.tableName}Table`, {
            tableName: `${definition.tableName}`,
            partitionKey: definition.partitionKey,
            sortKey: definition.sortKey,
            billingMode: definition.billingMode,
            pointInTimeRecovery: true,
        });

        if (definition.localSecondaryIndexes) {
            definition.localSecondaryIndexes.forEach((lsi) => table.addLocalSecondaryIndex(lsi));
        }

        if (definition.globalSecondaryIndexes) {
            definition.globalSecondaryIndexes.forEach((gsi) => table.addGlobalSecondaryIndex(gsi));
        }

        return table;
    }
}
