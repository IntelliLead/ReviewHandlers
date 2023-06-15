import {
    getEnvFromStackCreationInfo, IntellileadPipeline, IntelliLeadPipelineProps
} from "common-cdk";
import { Stack, StackProps } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { SERVICE_NAME } from '../constant';
import { DeploymentStacks } from './deployment-stacks';

export class PipelineStack extends Stack {

    constructor(scope: Construct, id: string, props?: StackProps) {
        super(scope, id, props);

        this.createPipeline();
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

        const pipeline = new IntellileadPipeline(
            this,
            `${SERVICE_NAME}-Pipeline`,
            pipelineProps,
        );

        pipeline.deploymentGroupCreationProps.forEach(stageProps => {
            const { stackCreationInfo } = stageProps;

            const deploymentStacks = new DeploymentStacks(
                pipeline,
                `${stackCreationInfo.stackPrefix}-DeploymentStacks`,
                {
                    stackCreationInfo,
                    env: getEnvFromStackCreationInfo(stackCreationInfo),
                },
            );

            pipeline.addDeploymentStage(
                stackCreationInfo,
                deploymentStacks,
            );
        });

        pipeline.pipeline.buildPipeline();

        return pipeline;
    }

}


