// import { Environment, Stack } from 'aws-cdk-lib';
// import { Construct } from 'constructs';
// import { StackCreationInfo } from 'vest-common-cdk';
// import { DnsStack } from './dns';
// import { Certificate } from 'aws-cdk-lib/aws-certificatemanager';
// import { createAcmCertificate } from '../../util';
// import { STATIC_SUBDOMAIN } from '../../constant';
//
// export interface USE1ResourcesStackProps {
//     readonly dns: DnsStack;    // dns is skipped for alpha stack
//     readonly env: Environment;
//     readonly crossRegionReferences: boolean;
//     readonly stackCreationInfo: StackCreationInfo;
//     readonly terminationProtection?: boolean;
// }
//
// export class USE1ResourcesStack extends Stack {
//     public readonly cloudFrontCertificate: Certificate;
//
//     constructor(scope: Construct, id: string, props: USE1ResourcesStackProps) {
//         super(scope, id, props);
//
//         const { dns } = props;
//         this.cloudFrontCertificate = createAcmCertificate(
//             this,
//             dns.serviceHostedZone,
//             `${props.stackCreationInfo.stackPrefix}-CloudFrontCertificate`,
//             `${STATIC_SUBDOMAIN}.${dns.serviceHostedZone.zoneName}`);
//     }
// }