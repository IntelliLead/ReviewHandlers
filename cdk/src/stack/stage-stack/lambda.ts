import { Duration, Stack } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { StackCreationInfo, STAGE } from 'common-cdk';
import { FunctionUrlAuthType, LambdaInsightsVersion, LayerVersion, Tracing } from 'aws-cdk-lib/aws-lambda';
import path from 'path';
import { DdbStack } from './ddb';
import { ManagedPolicy, PolicyStatement, Role, ServicePrincipal } from 'aws-cdk-lib/aws-iam';
import { VpcStack } from './vpc';
import { GoFunction } from '@aws-cdk/aws-lambda-go-alpha';
import { RetentionDays } from 'aws-cdk-lib/aws-logs';
import { FunctionUrl } from 'aws-cdk-lib/aws-lambda/lib/function-url';
import { StringParameter } from 'aws-cdk-lib/aws-ssm';
import { LambdaHandlerName } from '../../config/lambdaHandler';

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

// lambda functions type
export interface LambdaFunctions {
    [key: string]: GoFunction;
}

export class LambdaStack extends Stack {
    private readonly props: LambdaStackProps;
    public readonly lambdaFunctions: LambdaFunctions = {};

    constructor(scope: Construct, id: string, props: LambdaStackProps) {
        super(scope, id, props);
        this.props = props;

        const authRedirectUrlParameterName = '/auth/authRedirectUrl';

        this.lambdaFunctions[LambdaHandlerName.LINE_EVENTS_HANDLER] = this.createWebhookHandler(
            LambdaHandlerName.LINE_EVENTS_HANDLER,
            {
                AUTH_REDIRECT_URL_PARAMETER_NAME: authRedirectUrlParameterName,
            }
        ).lambdaFn;

        this.lambdaFunctions[LambdaHandlerName.NEW_REVIEW_EVENT_HANDLER] = this.createWebhookHandler(
            LambdaHandlerName.NEW_REVIEW_EVENT_HANDLER,
            {
                AUTH_REDIRECT_URL_PARAMETER_NAME: authRedirectUrlParameterName,
            }
        ).lambdaFn;

        const authHandlerWebhook = this.createWebhookHandler(LambdaHandlerName.AUTH_HANDLER, {
            AUTH_REDIRECT_URL_PARAMETER_NAME: authRedirectUrlParameterName,
        });
        this.lambdaFunctions[LambdaHandlerName.AUTH_HANDLER] = authHandlerWebhook.lambdaFn;

        // This would unfortunately create a circular dependency:
        // authHandler.lambdaFn.addEnvironment('AUTH_REDIRECT_URL', authHandler.functionUrl.url);
        // So instead we use SSM parameter store to store the auth redirect url and retrieve in runtime with Lambda extension
        // TODO: [INT-84] use Lambda extension to cache the value
        new StringParameter(this, 'authRedirectUrl', {
            parameterName: authRedirectUrlParameterName,
            stringValue: authHandlerWebhook.functionUrl.url,
            description: 'The auth handler lambda function url, used as Google OAuth2 redirect url',
        });
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
        handlerName: LambdaHandlerName,
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
        handlerRole.addToPolicy(this.buildGetParameterPolicy());
        handlerRole.addToPolicy(this.buildCloudwatchMetricPolicy());
        handlerRole.addManagedPolicy(ManagedPolicy.fromAwsManagedPolicyName('AWSXRayDaemonWriteAccess'));

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
            tracing: Tracing.ACTIVE,

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

    private buildGetParameterPolicy(): PolicyStatement {
        return new PolicyStatement({
            actions: ['ssm:GetParameter'],
            resources: ['*'],
        });
    }

    private buildCloudwatchMetricPolicy(): PolicyStatement {
        return new PolicyStatement({
            actions: ['cloudwatch:PutMetricData'],
            resources: ['*'],
        });
    }
}
