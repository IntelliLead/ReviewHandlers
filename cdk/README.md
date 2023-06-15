# api-core CDK infrastructure

This node module holds the service infrastructure managed with CDK.

## Set up your dev workspace

You will need AWS CLI (for credentials) and CDK CLI

1. Install AWS CLI following [here](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html).
   Stop after 'Install/Update' subsection
    - Ensure your default profile is set with your IAM user credentials. `cat ~/.aws/config && cat ~/.aws/credentials`
2. You will also need cdk cli. Test if `npx cdk --version` works.
3. For credentials, we use SSO. You will need to install AWS SSO CLI plugin. Follow [this](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sso.html) to set up your SSO profile.

## Developing this package

### Note on dependency

This node module depends on `vest-common-cdk`. You can find this in `dependencies` section of package.json.

This dependency is imported by fetching from Github and compiled to JS during `npm install`. If you need to make a
change in `vest-common-cdk`,

1. push it to a feature branch,
2. Modify `package.json` to track this branch. E.g., `"vest-common-cdk": "github:Vest/vest-common-cdk#YOUR_BRANCH",`
3. Update by `npm install vest-common-cdk`

p.s don't forget to also change the tracked branch under the dependency of package.json.

### Making infra changes for a deployed env

Infrastructure changes should first be deployed and tested in the alpha account of service.

1. Get creds for alpha account
    ```bash
   export DEV_ACCOUNT=${ALPHA_AWS_ACCOUNT_ID}
   aws configure sso
   . ../script/get-tmp-creds.sh
    ```
2. Compile CDK code with either commands
   ```
   npm run build  # just build current pkg
   npm run clean-build  # also pull latest vest-common-cdk changes by invoking npm install
   ``` 
3. Deploy the stacks you've updated
    ```
    npm run ls	# find [YOUR_STACK] from result
    npm run deploy [YOUR_STACK]
    ```
4. verify deployment in alpha account AWS console

### Making changes to pipeline itself

alpha is considered 'test env', and thus excluded from pipeline stages (beta, gamma, prod).

1. set DEV_ACCOUNT to the pipeline stack account (service beta account), get role profile
    ```bash
    export DEV_ACCOUNT=${ALPHA_AWS_ACCOUNT_ID}
    aws configure sso
    . ../script/get-tmp-creds.sh
    ```

   If you don't have permission to run script, First run `chmod +x ./script/*.sh`
2. Deploy pipeline

    ```
    unset DEV_ACCOUNT  # to tell CDK to generate pipeline stacks
    npm run build
    npm run ls	# find [PIPELINE_STACK] from result
    npm run deploy [PIPELINE_STACK]
    ```

### Setting up new accounts managed by pipeline

New AWS accounts require bootstrapping with CDK.

To verify if an account is boostrapped, view in AWS console if there is the Cloudformation stack CDKToolkit in the
region you specified.

Our convention is to deploy pipeline stack (resources) in service beta env.

1. Repeat bootstrap process for each account

    1. set dev-account role profile for the account to be bootstrapped

       ```
       export DEV_ACCOUNT=[ACCOUNT_NUMBER]
       . ../script/get-tmp-creds.sh
       ```

    3. bootstrapping

        1. for beta

           ```
           npx cdk bootstrap aws://$DEV_ACCOUNT/ap-northeast-1
           ```

        2. for others

           ```
           npx cdk bootstrap aws://$DEV_ACCOUNT/ap-northeast-1 \
               --cloudformation-execution-policies arn:aws:iam::aws:policy/AdministratorAccess \
               --trust [BETA_ACCOUNT_NUMBER]
           ```

### Troubleshooting

1. Cannot find `vest-common-cdk` during npm install
    1. either SSH or Github PAT need to be set.
2. ```
    Some context information is missing. Fetching...
    Retrieved account ID 083784680548 from disk cache
    Reading AZs for 083784680548:ap-northeast-1
    Assuming role 'arn:aws:iam::083784680548:role/cdk-hnb659fds-lookup-role-083784680548-ap-northeast-1'.
    Assuming role failed: User: arn:aws:sts::083784680548:assumed-role/api-core-alpha-Pipeline-PipelineBuildSynthCdkBuild-LX20ENLXM1AD/AWSCodeBuild-74699e0c-b95e-46c4-9317-8af9ea1f3642 is not authorized to perform: sts:AssumeRole on resource: arn:aws:iam::083784680548:role/cdk-hnb659fds-lookup-role-083784680548-ap-northeast-1
    Could not assume role in target account using current credentials User: arn:aws:sts::083784680548:assumed-role/api-core-alpha-Pipeline-PipelineBuildSynthCdkBuild-LX20ENLXM1AD/AWSCodeBuild-74699e0c-b95e-46c4-9317-8af9ea1f3642 is not authorized to perform: sts:AssumeRole on resource: arn:aws:iam::083784680548:role/cdk-hnb659fds-lookup-role-083784680548-ap-northeast-1 . Please make sure that this role exists in the account. If it doesn't exist, (re)-bootstrap the environment with the right '--trust', using the latest version of the CDK CLI.
    current credentials could not be used to assume 'arn:aws:iam::083784680548:role/cdk-hnb659fds-lookup-role-083784680548-ap-northeast-1', but are for the right account. Proceeding anyway.
    Call failed: describeAvailabilityZones(undefined) => You are not authorized to perform this operation. (code=UnauthorizedOperation)
    
    ```
   Ensure you have committed and pushed the VPC context for your deployment account, in `cdk.context.json`  by
   running `npx cdk synth` at least once for the deployment account. THis prevents VPC lookup during pipeline synth.

## Other Useful commands

* `npm run clean-build`   Re-install dependencies. Useful for vest-common-cdk or any internal dependency changes
* `npm run build`   compile typescript to js and synthesize CloudFormation template
* `npm run destroy -- [STACK_NAME]`  delete stack with [STACK_NAME] in AWS account
* `npx cdk diff`    compare deployed stack with current state
* `npx cdk ls`      list the synthesized CloudFormation stacks
* `npx cdk synth`   emits the synthesized CloudFormation template
