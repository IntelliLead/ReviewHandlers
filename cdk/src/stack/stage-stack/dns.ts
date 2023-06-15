// import {
//   DELEGATION_PARENT_DOMAIN,
//   getStagelessServiceAccountId, HOSTED_ZONE_DELEGATION_ROLE_NAME,
//   serviceDnsShortname,
//   StackCreationInfo,
//   STAGELESS_SERVICE,
// } from 'vest-common-cdk';
// import { Stack } from 'aws-cdk-lib';
// import { Certificate } from 'aws-cdk-lib/aws-certificatemanager';
// import { Role } from 'aws-cdk-lib/aws-iam';
// import { CrossAccountZoneDelegationRecord, PublicHostedZone } from 'aws-cdk-lib/aws-route53';
// import { Construct } from 'constructs';
// import { SERVICE_NAME } from '../../constant';
// import { createAcmCertificate } from '../../util';
//
//
// export interface DnsStackProps {
//   readonly stackCreationInfo: StackCreationInfo;
//   readonly crossRegionReferences: boolean;
//   readonly terminationProtection?: boolean;
// }
//
// export class DnsStack extends Stack {
//   public readonly serviceHostedZone: PublicHostedZone;
//   public readonly ecsCertificate: Certificate;
//
//
//   constructor(scope: Construct, id: string, props: DnsStackProps) {
//     super(scope, id, props);
//
//     this.serviceHostedZone = this.createDelegatedHostedZoneForService(props.stackCreationInfo);
//
//     this.ecsCertificate = createAcmCertificate(this, this.serviceHostedZone, `${ props.stackCreationInfo.stackPrefix }-EcsCertificate`);
//   }
//
//   // delegate from parent Hosted Zone in DNS account
//   private createDelegatedHostedZoneForService(stackCreationInfo: StackCreationInfo): PublicHostedZone {
//     const {
//       stage,
//       region,
//     } = stackCreationInfo;
//
//     const serviceDomain = `${ serviceDnsShortname.VestBot }.${ stage }.${ region }.${ DELEGATION_PARENT_DOMAIN }`;
//
//     const subZone = new PublicHostedZone(this, `${ stage }${ SERVICE_NAME }Subdomain`, {
//       zoneName: serviceDomain,
//     });
//
//     // import the delegation role by constructing the roleArn
//     const delegationRoleArn = Stack.of(this).formatArn({
//       region: '', // IAM is global in each partition
//       service: 'iam',
//       account: getStagelessServiceAccountId(STAGELESS_SERVICE.DNS),
//       resource: 'role',
//       resourceName: HOSTED_ZONE_DELEGATION_ROLE_NAME,
//     });
//     const delegationRole = Role.fromRoleArn(this, `${ SERVICE_NAME }HZDelegationRole`, delegationRoleArn);
//
//     // create the record
//     new CrossAccountZoneDelegationRecord(this, `${ SERVICE_NAME }HZDelegationRecord`, {
//       delegatedZone: subZone,
//       parentHostedZoneName: DELEGATION_PARENT_DOMAIN,
//       delegationRole,
//     });
//
//     return subZone;
//   }
//
// }
