import argparse

import boto3
dynamodb = boto3.client('dynamodb')

def get_all_users():
    response = dynamodb.scan(
        TableName='User')

    return response['Items']


def get_all_businesses():
    response = dynamodb.scan(
        TableName='Business')

    return response['Items']


def is_legacy_business_id(business_id: str) -> bool:
    # check if the format is accounts/106775638291982182570/locations/12251512170589559833
    parts = business_id.split('/')

    if len(parts) != 4 or parts[0] != 'accounts' or parts[2] != 'locations':
        return False

    if not parts[1].isnumeric() or not parts[3].isnumeric():
        return False

    return True


def get_new_business_id(business_id: str) -> str:
    parts = business_id.split('/')
    return parts[3]


def get_business_account_id(business_id: str) -> str:
    parts = business_id.split('/')
    return parts[1]


def update_business_account_id(origin: [str], skip_user: [bool], new: str, user_id: str):
    if not origin:
        print(f"ERROR: Origin cannot be empty for {user_id} business account id {new}")
        skip_user[0] = True
        return

    if origin[0]:
        if origin[0] != new:
            print(f"ERROR: User {user_id} has multiple business account ids: {origin[0]}, {new}")
            skip_user[0] = True

    origin[0] = new


def update_user_table(dry_run: bool):
    user_ids = get_all_users()

    for user in user_ids:
        # list is necessary to pass by reference
        skip_user_list = [False]
        business_account_id_list = [""]
        user_id = user['userId']['S']
        if 'businessIds' in user:
            # validate all businessIds should have the same 'account/xxx' prefix
            for business_id in user['businessIds']['SS']:
                if not is_legacy_business_id(business_id):
                    print(f"ERROR: User {user_id} has invalid business ID in businessIDs: {business_id}")
                    skip_user_list[0] = True
                else:
                    update_business_account_id(business_account_id_list, skip_user_list, get_business_account_id(business_id),
                                               user_id)
        else:
            print(f"WARN: User {user_id} has no businessIds")

        if 'activeBusinessId' in user:
            if not is_legacy_business_id(user['activeBusinessId']['S']):
                print(
                    f"ERROR: User {user_id} has invalid business ID in activeBusinessId: {user['activeBusinessId']['S']}")
                skip_user_list[0] = True
                return
            else:
                update_business_account_id(business_account_id_list, skip_user_list,
                                           get_business_account_id(user['activeBusinessId']['S']),
                                           user_id)
        else:
            print(f"WARN: User {user_id} has no activeBusinessId")

        if 'google' not in user:
            print(f"WARN: User {user_id} has no google attribute")
            skip_user_list[0] = True

        if skip_user_list[0]:
            print(f"WARN: Skipping user {user_id}")
            continue

        # prepare user attributes for update
        user['businessIds']['SS'] = [get_new_business_id(business_id) for business_id in
                                     user['businessIds']['SS']]
        user['activeBusinessId']['S'] = get_new_business_id(user['activeBusinessId']['S'])

        # add new attribute in user::google::businessAccountId
        user['google']['M']['businessAccountId'] = {'S': business_account_id_list[0]}

        if not dry_run:
            dynamodb.update_item(
                TableName='User',
                Key={
                    'userId': {'S': user['userId']['S']},
                    'uniqueId': {'S': user['uniqueId']['S']}
                },
                UpdateExpression='SET businessIds = :businessIds, activeBusinessId = :activeBusinessId, google = :google',
                ExpressionAttributeValues={
                    ':businessIds': user['businessIds'],
                    ':activeBusinessId': user['activeBusinessId'],
                    ':google': user['google']
                }
            )
        else:
            print(
                f"Dry run, would have updated user {user_id} with:\n"
                f"user.activeBusinessId from to {user['activeBusinessId']['S']}\n"
                f"user.businessIds to {user['businessIds']['SS']}\n"
                f"user.google.businessAccountId to {user['google']['M']['businessAccountId']['S']}\n"
            )


def update_business_table(dry_run: bool):
    businesses = get_all_businesses()

    for business in businesses:
        business_id = business['businessId']['S']
        if not is_legacy_business_id(business_id):
            print(f"ERROR: Business {business_id} has invalid business ID. Skipping.")
            continue

        # prepare business attributes for update
        business['businessId']['S'] = get_new_business_id(business_id)

        if not dry_run:
            # use transaction to ensure atomicity
            dynamodb.transact_write_items(
                TransactItems=[
                    {
                        'Put': {
                            'TableName': 'Business',
                            'Item': business
                        }
                    },
                    {
                        'Delete': {
                            'TableName': 'Business',
                            'Key': {'businessId': {'S': business_id}, 'uniqueId': {'S': business['uniqueId']['S']}}
                        }
                    },
                ]
            )
        else:
            print(f"Dry run, would have deleted business with key {business_id} and created new business item with key {business['businessId']['S']}")


def main(dry_run=False):
    update_user_table(dry_run)
    update_business_table(dry_run)


if __name__ == "__main__":
    if __name__ == "__main__":
        parser = argparse.ArgumentParser(description='Shorten business IDs in user and business table')
    parser.add_argument('--dry-run', action='store_true', help='Print changes without modifying the database')
    args = parser.parse_args()

    main(dry_run=args.dry_run)
