import { Stack } from 'aws-cdk-lib';
import {
    FlowLogDestination,
    FlowLogMaxAggregationInterval,
    GatewayVpcEndpointAwsService,
    IpAddresses,
    SubnetType,
    Vpc,
} from 'aws-cdk-lib/aws-ec2';
import { AnyPrincipal, PolicyStatement } from 'aws-cdk-lib/aws-iam';
import { Construct } from 'constructs';
import { StackCreationInfo } from 'common-cdk';

export interface VpcStackProps {
    readonly stackCreationInfo: StackCreationInfo;
    readonly terminationProtection?: boolean;
}

export class VpcStack extends Stack {
    public readonly vpc: Vpc;

    constructor(scope: Construct, id: string, props: VpcStackProps) {
        super(scope, id, props);

        this.vpc = new Vpc(this, `${props.stackCreationInfo.stackPrefix}-Vpc`, {
            ipAddresses: IpAddresses.cidr('10.0.0.0/16'),
            natGateways: 0,
            maxAzs: Stack.of(this).availabilityZones.length,
            subnetConfiguration: [
                {
                    cidrMask: 20,
                    subnetType: SubnetType.PUBLIC,
                    name: 'Public',
                },
                {
                    cidrMask: 20,
                    subnetType: SubnetType.PRIVATE_ISOLATED,
                    name: 'Isolated',
                },
            ],
        });

        this.vpc.addFlowLog('FlowLogCloudWatch', {
            destination: FlowLogDestination.toCloudWatchLogs(),
            // TODO: downgrade to S3 for half price https://aws.amazon.com/cloudwatch/pricing/#:~:text=0.01%20per%20minute-,Vended%20Logs,-Vended%20logs%20are
            // destination: FlowLogDestination.toS3(),
            maxAggregationInterval: FlowLogMaxAggregationInterval.TEN_MINUTES,
        });

        const dynamoGatewayEndpoint = this.vpc.addGatewayEndpoint('DynamoDBGatewayEndpoint', {
            service: GatewayVpcEndpointAwsService.DYNAMODB,
        });
        dynamoGatewayEndpoint.addToPolicy(
            new PolicyStatement({
                principals: [new AnyPrincipal()],
                actions: ['dynamodb:*'],
                resources: ['*'],
            })
        );

        const s3GatewayEndpoint = this.vpc.addGatewayEndpoint('S3GatewayEndpoint', {
            service: GatewayVpcEndpointAwsService.S3,
        });
        s3GatewayEndpoint.addToPolicy(
            new PolicyStatement({
                principals: [new AnyPrincipal()],
                actions: ['s3:*'],
                resources: ['*'],
            })
        );
    }
}
