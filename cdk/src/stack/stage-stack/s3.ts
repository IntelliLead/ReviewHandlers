// import { Duration, RemovalPolicy, Stack } from 'aws-cdk-lib';
// import { BlockPublicAccess, Bucket, BucketEncryption, ObjectOwnership } from 'aws-cdk-lib/aws-s3';
// import { Construct } from 'constructs';
// import { StackCreationInfo } from 'vest-common-cdk';
// import { SERVICE_NAME } from '../../constant';
// import { OriginAccessIdentity } from 'aws-cdk-lib/aws-cloudfront';
//
// export interface S3StackProps {
//     readonly stackCreationInfo: StackCreationInfo;
//     readonly terminationProtection?: boolean;
// }
//
// export class S3Stack extends Stack {
//     public readonly elbAccessLogBucket: Bucket;
//     public readonly cloudFrontLogBucket: Bucket;
//     public readonly staticContentBucket: Bucket;    // served with CDN
//     public readonly originAccessIdentity: OriginAccessIdentity;
//     private readonly props: S3StackProps;
//
//     constructor(scope: Construct, id: string, props: S3StackProps) {
//         super(scope, id, props);
//         this.props = props;
//
//         this.elbAccessLogBucket = this.createBucket('elb-access-log', false, true);
//         this.cloudFrontLogBucket = this.createBucket('cloudfront-log', false, true);
//         this.staticContentBucket = this.createBucket('static-content');
//
//         // CDK does not yet support Origin Access Control (recommended) for CloudFront
//         // https://github.com/aws/aws-cdk/issues/21771
//         this.originAccessIdentity = new OriginAccessIdentity(this, 'OriginAccessIdentity');
//         this.staticContentBucket.grantRead(this.originAccessIdentity);
//     }
//
//     private createBucket(name: string, versioned = true, logging = false): Bucket {
//         const bucketFullName = this.getBucketName(name);
//
//         return new Bucket(this, bucketFullName, {
//             encryption: BucketEncryption.S3_MANAGED,
//             bucketName: bucketFullName,
//             removalPolicy: RemovalPolicy.DESTROY,
//             blockPublicAccess: BlockPublicAccess.BLOCK_ALL,
//             objectOwnership: ObjectOwnership.BUCKET_OWNER_PREFERRED,
//             versioned: versioned,
//             ...(logging && {
//                 lifecycleRules: [ {
//                     id: 'limit max log age',
//                     expiration: Duration.days(6 * 30),
//                 } ],
//             }),
//         });
//     }
//
//     private getBucketName(name: string): string {
//         return `${this.props.stackCreationInfo.stackPrefix}-${SERVICE_NAME}-${name}`.toLowerCase();
//     }
//
// }