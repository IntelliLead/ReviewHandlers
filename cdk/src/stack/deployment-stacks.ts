import { Environment, StackProps, Stage } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { StackCreationInfo, STAGE } from 'common-cdk';
import { VpcStack } from './stage-stack/vpc';
import { DdbStack } from './stage-stack/ddb';
import { SecretStack } from './stage-stack/secret';
import { LambdaStack } from './stage-stack/lambda';
import { CloudwatchStack } from './stage-stack/cloudwatch';

export interface DeploymentStacksProps extends StackProps {
    readonly stackCreationInfo: StackCreationInfo;
    readonly env: Environment;
}

export class DeploymentStacks extends Stage {
    public readonly vpc: VpcStack;
    public readonly ddb: DdbStack;
    public readonly lambda: LambdaStack;
    public readonly secret: SecretStack;
    public readonly cloudwatch: CloudwatchStack;

    constructor(scope: Construct, id: string, props: DeploymentStacksProps) {
        super(scope, id, props);

        const { stackCreationInfo } = props;
        const { stackPrefix, stage } = stackCreationInfo;

        const terminationProtection = stage !== STAGE.ALPHA; // Termination protection for non-DEV envs
        // const enableHttps = stage !== STAGE.ALPHA;
        // const deploySecret = stage !== STAGE.ALPHA;   // Secret deployed for non-DEV envs. Alpha uses beta secrets

        this.vpc = new VpcStack(this, `${stackPrefix}-Vpc`, {
            stackCreationInfo,
            terminationProtection,
        });

        this.ddb = new DdbStack(this, `${stackPrefix}-Ddb`, {
            stackCreationInfo,
            terminationProtection,
        });

        this.lambda = new LambdaStack(this, `${stackPrefix}-Lambda`, {
            stackCreationInfo,
            vpc: this.vpc,
            ddb: this.ddb,
            terminationProtection,
        });

        this.secret = new SecretStack(this, `${stackPrefix}-Secret`, {
            stackCreationInfo,
            terminationProtection,
        });
        this.lambda.addDependency(this.secret);

        this.cloudwatch = new CloudwatchStack(this, `${stackPrefix}-Cloudwatch`, {
            stackCreationInfo,
            lambdas: this.lambda,
            terminationProtection,
        });

        // }
        //
        // const use1StackCreationInfo = stackCreationInfo;
        // use1StackCreationInfo.region = 'us-east-1';
        // if (stage !== STAGE.ALPHA) {
        //     this.use1Resources = new USE1ResourcesStack(this, `${stackPrefix}-USE1Resources`, {
        //         dns: this.dns!,
        //         env: {
        //             account: props.env.account,
        //             region: 'us-east-1',
        //         },
        //         crossRegionReferences: true,
        //         stackCreationInfo: use1StackCreationInfo,
        //         terminationProtection,
        //     });
        // }
        //
        // this.cloudfront = new CloudFrontStack(this, `${stackPrefix}-CloudFront`, {
        //     cloudFrontCertificate: this.use1Resources?.cloudFrontCertificate,
        //     dnsStack: this.dns,
        //     s3Stack: this.s3,
        //     stackCreationInfo,
        //     crossRegionReferences: true,
        //     terminationProtection,
        // });
    }
}
