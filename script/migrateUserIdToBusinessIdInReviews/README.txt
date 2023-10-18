# Migrate UserId to BusinessId for Review

Context: https://linear.app/intellilead/issue/INT-86/refactor-and-backfill-review-table#comment-eaebbcfb

It should be modified before used again for other purposes.

## SOP
Preparation:
1. Install dependencies
    ```
    python3 -m venv env
    source env/bin/activate
    pip install boto3
    ```


In each stage:
1. Obtain stage environment credentials
    ```
    export DEV_ACCOUNT={STAGE_ACCOUNT}
    . ../get-tmp-creds.sh
    ```
2. Run script with dry-run mode to verify the data to be migrated
    ```
    python consolidate-review-id.py --dry-run | tee >(pbcopy)
    ```
3. Run script without dry-run mode to make the change
4. Repeat for next stage

