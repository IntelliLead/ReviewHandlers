import { StackCreationInfo } from 'common-cdk';
import { Stack } from 'aws-cdk-lib';
import { Table } from 'aws-cdk-lib/aws-dynamodb';
import { IRole } from 'aws-cdk-lib/aws-iam';
import { Construct } from 'constructs';

import { DynamoDbTableAttribute, TableName, Tables } from '../../config/tables';

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

        Tables.forEach((table) => {
            this.tableEntries.set(table.tableName, this.createTable(table));
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
