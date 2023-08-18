import { Duration, Stack } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { ORGANIZATION_ID, StackCreationInfo, STAGE } from 'common-cdk';
import {
    Code,
    FunctionUrlAuthType,
    LambdaInsightsVersion,
    LayerVersion,
    ParamsAndSecretsVersions,
    Runtime,
} from 'aws-cdk-lib/aws-lambda';
import path from 'path';
import { DdbStack } from './ddb';
import {
    AccountRootPrincipal,
    ManagedPolicy,
    OrganizationPrincipal,
    PolicyStatement,
    Role,
    ServicePrincipal,
} from 'aws-cdk-lib/aws-iam';
import { VpcStack } from './vpc';
import { GoFunction } from '@aws-cdk/aws-lambda-go-alpha';
import { RetentionDays } from 'aws-cdk-lib/aws-logs';
import { FunctionUrl } from 'aws-cdk-lib/aws-lambda/lib/function-url';
import { StringParameter } from 'aws-cdk-lib/aws-ssm';

export interface LambdaStackProps {
    readonly stackCreationInfo: StackCreationInfo;
    readonly vpc: VpcStack;
    readonly ddb: DdbStack;
    readonly terminationProtection?: boolean;
}

type EnvObject = {
    [key: string]: string | undefined;
};

interface WebhookHandler {
    readonly lambdaFn: GoFunction;
    readonly functionUrl: FunctionUrl;
}

export class LambdaStack extends Stack {
    private readonly props: LambdaStackProps;

    constructor(scope: Construct, id: string, props: LambdaStackProps) {
        super(scope, id, props);
        this.props = props;
        const { stage } = this.props.stackCreationInfo;

        this.createWebhookHandler('lineEventsHandler');
        this.createWebhookHandler('newReviewEventHandler');

        const authRedirectUrlParameterName = '/auth/authRedirectUrl';
        const authHandler = this.createWebhookHandler('AuthHandler', {
            AUTH_REDIRECT_URL_PARAMETER_NAME: authRedirectUrlParameterName,
        });

        // This unfortunately creates a circular dependency
        // authHandler.lambdaFn.addEnvironment('AUTH_REDIRECT_URL', authHandler.functionUrl.url);
        // So instead we use SSM parameter store to store the auth redirect url and retrieve in runtime with Lambda extension
        // TODO: [INT-84] use Lambda extension to cache the value
        new StringParameter(this, 'authRedirectUrl', {
            parameterName: authRedirectUrlParameterName,
            stringValue: authHandler.functionUrl.url,
            description: 'The auth handler lambda function url, used as Google OAuth2 redirect url',
        });
        authHandler.lambdaFn.role?.addToPrincipalPolicy(
            new PolicyStatement({
                actions: ['ssm:GetParameter'],
                resources: ['*'],
            })
        );
        //
        // this.createWebhookHandler('tst', {
        //     AUTH_REDIRECT_URL: authHandler.functionUrl.url,
        // });
    }

    /**
     * Create Go Lambda function with FunctionUrl
     * handlerName must be src/cmd/{handlerName}/main.go
     *
     * @param handlerName
     * @param additionalEnv
     * @param layers - layers to be added to the function
     * @private
     */
    private createWebhookHandler(
        handlerName: string,
        additionalEnv: EnvObject = {},
        ...layers: LayerVersion[]
    ): WebhookHandler {
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
                ...additionalEnv,
            },
            bundling: {
                goBuildFlags: ['-ldflags "-s -w"'],
            },
            layers: [...layers],
            role: handlerRole,
            memorySize: 256,
            timeout: Duration.minutes(5),
            insightsVersion: LambdaInsightsVersion.VERSION_1_0_143_0,
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

        const functionUrl = handlerFunction.addFunctionUrl({
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

        return {
            lambdaFn: handlerFunction,
            functionUrl: functionUrl,
        };
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
