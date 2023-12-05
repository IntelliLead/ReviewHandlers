import { Stack } from 'aws-cdk-lib';
import { OrganizationPrincipal } from 'aws-cdk-lib/aws-iam';
import { Key } from 'aws-cdk-lib/aws-kms';
import { Secret } from 'aws-cdk-lib/aws-secretsmanager';
import { Construct } from 'constructs';
import { StackCreationInfo, ORGANIZATION_ID } from 'common-cdk';
import { SERVICE_NAME } from '../../constant';

export interface SecretStackProps {
    readonly stackCreationInfo: StackCreationInfo;
    readonly terminationProtection?: boolean;
}

export class SecretStack extends Stack {
    constructor(scope: Construct, id: string, props: SecretStackProps) {
        super(scope, id, props);

        const orgPrincipal = new OrganizationPrincipal(ORGANIZATION_ID);

        const serviceKeyAlias = `${SERVICE_NAME}Key`;
        const serviceKey = new Key(this, serviceKeyAlias, {
            alias: serviceKeyAlias,
        });
        serviceKey.grantEncryptDecrypt(orgPrincipal);

        const secret = new Secret(this, `${SERVICE_NAME}Secrets`, {
            encryptionKey: serviceKey,
            secretName: `${SERVICE_NAME}/secrets`,
            description: `${SERVICE_NAME} secrets`,
        });

        // Add cross-account access to server secret to allow alpha to use beta server secret
        // Org principal is automatically added to Secret resource policy and KMS Key policy for cross account access
        secret.grantRead(orgPrincipal);
    }

}