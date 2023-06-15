// import { Stack } from 'aws-cdk-lib';
// import { Construct } from 'constructs';
// import { StackCreationInfo, STAGE } from 'vest-common-cdk';
// import { S3Origin } from 'aws-cdk-lib/aws-cloudfront-origins';
// import { AllowedMethods, Distribution } from 'aws-cdk-lib/aws-cloudfront';
// import { DnsStack } from './dns';
// import { S3Stack } from './s3';
// import { BucketDeployment, Source } from 'aws-cdk-lib/aws-s3-deployment';
// import { Certificate } from 'aws-cdk-lib/aws-certificatemanager';
// import { STATIC_SUBDOMAIN } from '../../constant';
//
// export interface CloudFrontStackProps {
//     readonly cloudFrontCertificate?: Certificate;    // dns is skipped for alpha stack
//     readonly dnsStack?: DnsStack;    // dns is skipped for alpha stack
//     readonly s3Stack: S3Stack;
//     readonly stackCreationInfo: StackCreationInfo;
//     readonly crossRegionReferences: boolean;
//     readonly terminationProtection?: boolean;
// }
//
// export class CloudFrontStack extends Stack {
//     private readonly props: CloudFrontStackProps;
//
//     constructor(scope: Construct, id: string, props: CloudFrontStackProps) {
//         super(scope, id, props);
//         this.props = props;
//         const { dnsStack, s3Stack, stackCreationInfo } = props;
//         const { stage } = stackCreationInfo;
//         const { staticContentBucket, cloudFrontLogBucket, originAccessIdentity } = s3Stack;
//
//         // only available for non-alpha stages
//         const hostedZone = dnsStack?.serviceHostedZone;
//
//         const distribution = new Distribution(this, 'StaticContentDistribution', {
//             defaultBehavior: {
//                 origin: new S3Origin(staticContentBucket, {
//                     originAccessIdentity: originAccessIdentity,
//                 }),
//                 allowedMethods: AllowedMethods.ALLOW_GET_HEAD_OPTIONS
//             },
//             logBucket: cloudFrontLogBucket,
//             enableIpv6: true,
//             ...stage !== STAGE.ALPHA && {
//                 // even though this is supposed to also create the alias record of the custom domain name, it did not. It is currently done manually in Route53
//                 domainNames: [ `${STATIC_SUBDOMAIN}.${hostedZone!.zoneName}` ],
//                 certificate: props.cloudFrontCertificate!,
//             }
//         });
//
//         new BucketDeployment(this, 'DeployStaticContent', {
//             sources: [ Source.asset('../src/resources/static') ],
//             destinationBucket: staticContentBucket,
//             distribution,
//             prune: true,
//             distributionPaths: [ '/*' ],
//         });
//     }
//
// }