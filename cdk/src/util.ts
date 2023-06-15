import { PublicHostedZone } from 'aws-cdk-lib/aws-route53';
import { Certificate, CertificateValidation } from 'aws-cdk-lib/aws-certificatemanager';
import { Construct } from 'constructs';

export function createAcmCertificate(scope: Construct, hostedZone: PublicHostedZone, certificateName: string, domainName?: string): Certificate {
    return new Certificate(scope, certificateName, {
        domainName: domainName ?? hostedZone.zoneName,
        certificateName: certificateName,
        validation: CertificateValidation.fromDns(hostedZone),
    });
}