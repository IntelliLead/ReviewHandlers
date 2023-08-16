# consolidate Review ID
This script was used to consolidate the Review ID column in the dataset in observation of the [schema change](https://www.notion.so/intellilead/Engineering-Low-Level-Design-e643b1ccd23345d49fdffa4185cf73b7?pvs=4#bc755b62022246bdb17f1e9788b98efd).

It should be modified before used again for other purposes.

## SOP
1. Ensure no new reviews are coming in by deactivating (throttling) the Lambda function URL in all stages
2. Disable pipeline for every stage and enable the Lambda function URL in code change
In each stage:
3. Install dependencies
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

