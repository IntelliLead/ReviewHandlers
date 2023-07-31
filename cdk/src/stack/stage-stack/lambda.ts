import { Duration, Stack } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { StackCreationInfo, STAGE } from 'common-cdk';
import { Code, FunctionUrlAuthType, LambdaInsightsVersion, LayerVersion, Runtime } from 'aws-cdk-lib/aws-lambda';
import path from 'path';
import { DdbStack } from './ddb';
import { ManagedPolicy, PolicyStatement, Role, ServicePrincipal } from 'aws-cdk-lib/aws-iam';
import { VpcStack } from './vpc';
import { GoFunction } from '@aws-cdk/aws-lambda-go-alpha';
import { RetentionDays } from 'aws-cdk-lib/aws-logs';

export interface LambdaStackProps {
    readonly stackCreationInfo: StackCreationInfo;
    readonly vpc: VpcStack;
    readonly ddb: DdbStack;
    readonly terminationProtection?: boolean;
}

export class LambdaStack extends Stack {
    private readonly props: LambdaStackProps;

    constructor(scope: Construct, id: string, props: LambdaStackProps) {
        super(scope, id, props);
        this.props = props;

        const configLayer = this.createGCPConfigLayer();
        this.createWebhookHandler('lineEventsHandler', configLayer);
        this.createWebhookHandler('newReviewEventHandler');
    }

    private createGCPConfigLayer(): LayerVersion {
        // the Workload identity federation config file
        // https://cloud.google.com/docs/authentication/application-default-credentials#GAC
        // https://cloud.google.com/iam/docs/workload-identity-federation-with-other-clouds#use_the_credential_configuration_to_access
        return new LayerVersion(this, 'GcpConfigLayer', {
            description: 'workload identity federation config file used for Google APIs',
            code: Code.fromAsset(path.join(__dirname, '../../../../config')),
        });
    }

    /**
     * Create Go Lambda function with FunctionUrl
     * handlerName must be src/cmd/{handlerName}/main.go
     *
     * @param handlerName
     * @param layers - layers to be added to the function
     * @private
     */
    private createWebhookHandler(handlerName: string, ...layers: LayerVersion[]) {
        const { stage } = this.props.stackCreationInfo;

        const handlerRole = new Role(this, `${handlerName}Role`, {
            assumedBy: new ServicePrincipal('lambda.amazonaws.com'),
        });
        handlerRole.addManagedPolicy(
            ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaBasicExecutionRole')
        );

        Array.from(this.props.ddb.tableEntries.values()).forEach((table) => {
            DdbStack.grantTable(table, handlerRole);
        });
        handlerRole.addToPolicy(this.buildGetSecretPolicy());
        handlerRole.addToPolicy(this.buildKmsDecryptPolicy());

        const handlerFunction = new GoFunction(this, handlerName, {
            entry: path.join(__dirname, `../../../../src/cmd/${handlerName}/main.go`),
            environment: {
                STAGE: stage,
            },
            bundling: {
                goBuildFlags: ['-ldflags "-s -w"'],
            },
            layers: [...layers],
            role: handlerRole,
            memorySize: 256,
            timeout: Duration.minutes(5),
            insightsVersion: LambdaInsightsVersion.VERSION_1_0_143_0,
            deadLetterQueueEnabled: true,
            logRetention: RetentionDays.SIX_MONTHS,
            // TODO: INT-47 enable tracing
            // tracing: Tracing.ACTIVE,

            // Lambda cannot reach Internet even in public subnet without NAT gateway
            // https://stackoverflow.com/a/52994841
            // don't run Lambda in VPC unless you need it to access private subnet resource
            // vpc: this.props.vpc.vpc,
            // vpcSubnets: {
            //     subnetType: SubnetType.PUBLIC,
            // },
        });

        handlerFunction.addFunctionUrl({
            authType: FunctionUrlAuthType.NONE,
            ...((stage == STAGE.PROD || stage == STAGE.GAMMA) && {
                cors: {
                    // TODO: tighten
                    allowedOrigins: ['*'],
                },
            }),
        });

        // TODO: INT-48 create timeout metrics
        // if (fn.timeout) {
        //     new cloudwatch.Alarm(this, `MyAlarm`, {
        //         metric: fn.metricDuration().with({
        //             statistic: 'Maximum',
        //         }),
        //         evaluationPeriods: 1,
        //         datapointsToAlarm: 1,
        //         threshold: fn.timeout.toMilliseconds(),
        //         treatMissingData: cloudwatch.TreatMissingData.IGNORE,
        //         alarmName: 'My Lambda Timeout',
        //     });
        // }
    }

    private buildGetSecretPolicy(): PolicyStatement {
        return new PolicyStatement({
            actions: ['secretsmanager:GetSecretValue', 'secretsmanager:DescribeSecret'],
            resources: ['*'],
        });
    }

    private buildKmsDecryptPolicy(): PolicyStatement {
        return new PolicyStatement({
            actions: ['kms:Decrypt'],
            resources: ['*'],
        });
    }
}
