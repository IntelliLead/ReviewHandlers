import { StackCreationInfo, STAGE } from 'common-cdk';
import { Duration, Stack } from 'aws-cdk-lib';
import { LambdaStack } from './lambda';
import { Construct } from 'constructs';
import { Topic } from 'aws-cdk-lib/aws-sns';
import { SlackChannelConfiguration } from 'aws-cdk-lib/aws-chatbot';
import { GoFunction } from '@aws-cdk/aws-lambda-go-alpha';
import { Dashboard, GraphWidget, Metric, TextWidget, TreatMissingData } from 'aws-cdk-lib/aws-cloudwatch';
import { SnsAction } from 'aws-cdk-lib/aws-cloudwatch-actions';
import { PREPROD_SLACK_CHANNEL_ID, PROD_SLACK_CHANNEL_ID, SLACK_WORKSPACE_ID } from '../../constant';
import { ManagedPolicy } from 'aws-cdk-lib/aws-iam';

export interface CloudwatchStackProps {
    readonly stackCreationInfo: StackCreationInfo;
    readonly lambdas: LambdaStack;
    readonly terminationProtection?: boolean;
}

export class CloudwatchStack extends Stack {
    constructor(scope: Construct, id: string, props: CloudwatchStackProps) {
        super(scope, id, props);
        const { stage } = props.stackCreationInfo;
        const lambdaFunctions = props.lambdas.lambdaFunctions;

        const alarmTopic = new Topic(this, 'LambdaAlarmTopic');

        const slackConfig = new SlackChannelConfiguration(this, 'SlackChannelConfig', {
            slackChannelConfigurationName: 'SlackChannelConfig',
            slackWorkspaceId: SLACK_WORKSPACE_ID,
            slackChannelId: stage == STAGE.PROD ? PROD_SLACK_CHANNEL_ID : PREPROD_SLACK_CHANNEL_ID,
            notificationTopics: [alarmTopic],
        });
        slackConfig.role?.addManagedPolicy(ManagedPolicy.fromAwsManagedPolicyName('CloudWatchReadOnlyAccess'));

        const dashboard = new Dashboard(this, 'MetricsDashboard', {
            dashboardName: 'MetricsDashboard',
            start: '-PT30M', // Start time for the dashboard, e.g., last 30 minutes
        });

        for (const [name, lambdaFn] of Object.entries(lambdaFunctions)) {
            this.addMetricAlarmsToDashboardForLambda(name, lambdaFn, alarmTopic, dashboard);
        }
    }

    private addMetricAlarmsToDashboardForLambda(
        handlerName: string,
        lambdaFn: GoFunction,
        alarmTopic: Topic,
        dashboard: Dashboard
    ) {
        // 4xx errors
        const metric4xx = new Metric({
            metricName: '4XXError',
            namespace: 'AWS/Lambda',
            dimensionsMap: { FunctionName: handlerName },
            statistic: 'Sum',
            period: Duration.minutes(5),
        });

        metric4xx
            .createAlarm(this, `${handlerName}-4xxAlarm`, {
                threshold: 1,
                evaluationPeriods: 1,
                treatMissingData: TreatMissingData.NOT_BREACHING,
                actionsEnabled: true,
            })
            .addAlarmAction(new SnsAction(alarmTopic));

        // 5xx errors
        const metric5xx = new Metric({
            metricName: '5XXError',
            namespace: 'AWS/Lambda',
            dimensionsMap: { FunctionName: handlerName },
            statistic: 'Sum',
            period: Duration.minutes(5),
        });
        metric5xx
            .createAlarm(this, `${handlerName}-5xxAlarm`, {
                threshold: 1,
                evaluationPeriods: 1,
                treatMissingData: TreatMissingData.NOT_BREACHING,
                actionsEnabled: true,
            })
            .addAlarmAction(new SnsAction(alarmTopic));

        // Latency
        const metricLatency = lambdaFn.metricDuration();
        metricLatency
            .createAlarm(this, `${handlerName}-LatencyAlarm`, {
                threshold: 10000, // 10 seconds in milliseconds
                evaluationPeriods: 1,
                treatMissingData: TreatMissingData.NOT_BREACHING,
                actionsEnabled: true,
            })
            .addAlarmAction(new SnsAction(alarmTopic));

        dashboard.addWidgets(
            new TextWidget({
                markdown: `## ${handlerName} Metrics`,
                width: 24, // Spanning the width to cover all four graphs
            })
        );

        dashboard.addWidgets(
            new GraphWidget({
                title: `Invocations`,
                left: [lambdaFn.metricInvocations()],
                width: 6,
            }),
            new GraphWidget({
                title: `4XX Errors`,
                left: [metric4xx],
                width: 6,
            }),
            new GraphWidget({
                title: `5XX Errors`,
                left: [metric5xx],
                width: 6,
            }),
            new GraphWidget({
                title: `Latency`,
                left: [metricLatency],
                width: 6,
            })
        );
    }
}
