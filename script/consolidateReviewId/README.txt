# consolidate Review ID
This script was used to copy activeBusinessId to []businessIds for all users.

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

