import argparse
import boto3

dynamodb = boto3.client('dynamodb')


def scan_and_update_users(dry_run):
    response = dynamodb.scan(
        TableName='User',
    )

    users_without_active_business_id = 0
    user_updated = 0
    total_users = len(response['Items'])

    for user in response['Items']:
        user_id = user['userId']['S']

        user_name = "NO_LINE_USER_NAME"
        if 'lineUsername' in user and user['lineUsername']:
            user_name = user['lineUsername']['S']

        if 'activeBusinessId' in user and user['activeBusinessId']['S']:
            active_business_id = user['activeBusinessId']['S']

            if dry_run:
                print(
                    f"[DRY RUN] Would update user {user_id} ({user_name}) with businessIds set to [{active_business_id}].")
            else:
                update_response = dynamodb.update_item(
                    TableName='User',
                    Key={
                        'userId': {'S': user_id},
                        'uniqueId': {'S': "#"},
                    },
                    UpdateExpression="SET businessIds = :newBusinessIds",
                    ExpressionAttributeValues={":newBusinessIds": {'SS': [active_business_id]}}
                )

                if update_response['ResponseMetadata']['HTTPStatusCode'] != 200:
                    print(f"Failed to update user {user_id} with business ID {active_business_id}.")
                else:
                    user_updated += 1
        else:
            users_without_active_business_id += 1
            print(f"User {user_id} ({user_name}) does not have activeBusinessId attribute.")

    print(f"Total users scanned: {total_users}")
    print(f"Total users updated: {user_updated}")
    print(f"Users without activeBusinessId: {users_without_active_business_id}")


def main(dry_run):
    scan_and_update_users(dry_run)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Update users with businessIds attribute.')
    parser.add_argument('--dry-run', action='store_true', help='Print changes without modifying the database')
    args = parser.parse_args()

    main(dry_run=args.dry_run)
