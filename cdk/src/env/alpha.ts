import { App } from 'aws-cdk-lib';
import {
  DEFAULT_REGION,
  getEnvFromStackCreationInfo,
  STAGE,
  createStackCreationInfo,
} from 'common-cdk';
import { DeploymentStacks } from '../stack/deployment-stacks';

export function createAlphaStacks(app: App, devAccountId: string) {
  const stackCreationInfo = createStackCreationInfo(devAccountId, DEFAULT_REGION, STAGE.ALPHA);

  new DeploymentStacks(app, `${ stackCreationInfo.stackPrefix }-DeploymentStacks`, {
    stackCreationInfo,
    env: getEnvFromStackCreationInfo(stackCreationInfo),
  });

}
