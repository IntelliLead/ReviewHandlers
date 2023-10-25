import { getEnvFromStackCreationInfo, IntellileadPipeline, IntelliLeadPipelineProps } from 'common-cdk';
import { Stack, StackProps } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { PREPROD_SLACK_CHANNEL_ID, SERVICE_NAME, SLACK_WORKSPACE_ID } from '../constant';
import { DeploymentStacks } from './deployment-stacks';
import { Topic } from 'aws-cdk-lib/aws-sns';
import { SlackChannelConfiguration } from 'aws-cdk-lib/aws-chatbot';
import { DetailType, NotificationRule } from 'aws-cdk-lib/aws-codestarnotifications';

export class PipelineStack extends Stack {
    constructor(scope: Construct, id: string, props?: StackProps) {
        super(scope, id, props);

        const pipeline = this.createPipeline();

        const notificationTopic = new Topic(this, 'PipelineNotificationTopic');

        new SlackChannelConfiguration(this, 'PipelineSlackChannelConfig', {
            slackChannelConfigurationName: 'PipelineSlackChannelConfig',
            slackWorkspaceId: SLACK_WORKSPACE_ID,
            slackChannelId: PREPROD_SLACK_CHANNEL_ID,
            notificationTopics: [notificationTopic],
        });

        const rule = new NotificationRule(this, 'PipelineNotification', {
            detailType: DetailType.BASIC,
            events: ['codepipeline-pipeline-pipeline-execution-failed'],
            source: pipeline.pipeline.pipeline,
            targets: [notificationTopic],
        });
        rule.addTarget(notificationTopic);
    }

    private createPipeline(): IntellileadPipeline {
        const pipelineProps: IntelliLeadPipelineProps = {
            service: SERVICE_NAME,
            prodManualApproval: false,
            trackingPackages: [
                {
                    package: 'ReviewHandlers',
                    branch: 'main',
                },
                {
                    package: 'CommonCDK',
                    branch: 'main',
                },
            ],
        };

        const pipeline = new IntellileadPipeline(this, `${SERVICE_NAME}-Pipeline`, pipelineProps);

        pipeline.deploymentGroupCreationProps.forEach((stageProps) => {
            const { stackCreationInfo } = stageProps;

            const deploymentStacks = new DeploymentStacks(
                pipeline,
                `${stackCreationInfo.stackPrefix}-DeploymentStacks`,
                {
                    stackCreationInfo,
                    env: getEnvFromStackCreationInfo(stackCreationInfo),
                }
            );

            pipeline.addDeploymentStage(stackCreationInfo, deploymentStacks);
        });

        pipeline.pipeline.buildPipeline();

        return pipeline;
    }
}
