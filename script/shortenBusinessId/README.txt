# Shorten Business ID
This script was used to shorten the business ID for all the businesses in the database. It was used to migrate the business ID from long format
'account/123/locations/xxx' to 'xxx', and backfill the '123' in account/123' portion to user's Google::businessAccountId field.

It should be modified before used again for other purposes.

## SOP
1. In each stage:
    Install dependencies
    ```
    python3 -m venv env
    source env/bin/activate
    pip install boto3
    ```

3. Obtain stage environment credentials
    ```
    export DEV_ACCOUNT={STAGE_ACCOUNT}
    . ../get-tmp-creds.sh
    ```
3. Run script with dry-run mode to verify the data to be migrated
4. Run script without dry-run mode to make the change
5. Enable pipeline for that stage
6. Repeat for next stage

