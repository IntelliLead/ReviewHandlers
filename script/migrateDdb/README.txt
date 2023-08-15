# migrate DDB scripts

This script was used in a one-time scenario to migrate user and review data from open-beta preview AWS account to prod account.

It should be modified before used again for other purposes.


## SOP
1. Ensure no new reviews are coming in by disabling the Lambda function URL
2. Run script
3. Enable the Lambda function URL