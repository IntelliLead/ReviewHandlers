import { App } from 'aws-cdk-lib';
import { SERVICE_NAME } from '../constant';
import { PipelineStack } from '../stack/pipeline';
import { STAGE, stageEnvironmentConfiguration, DEFAULT_REGION } from 'common-cdk';

export function createPipeline(app: App) {
  const pipelineAccountInfo = stageEnvironmentConfiguration[STAGE.BETA];
  const pipelineAccountId = pipelineAccountInfo.accountId;

  new PipelineStack(app, `${SERVICE_NAME}-Pipeline`, {
    env: {
      region: DEFAULT_REGION,
      account: pipelineAccountId,
    },
  });

}
