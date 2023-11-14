import argparse

import boto3
from boto3.dynamodb.conditions import Key

dynamodb = boto3.client('dynamodb')


def get_all_reviews():
    response = dynamodb.scan(
        TableName='Review')

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


def update_review_table(dry_run: bool):
    reviews = get_all_reviews()

    for review in reviews:
        old_business_id = review['userId']['S']
        review_id = review['uniqueId']['S']

        # if reviewId begins with '#UNIQUE_VENDOR_REVIEW_ID#', then it should be deleted
        if review_id.startswith('#UNIQUE_VENDOR_REVIEW_ID#'):
            if not dry_run:
                dynamodb.delete_item(
                    TableName='Review',
                    Key={'userId': {'S': old_business_id}, 'uniqueId': {'S': review_id}}
                )
                print(f"Deleted unique vendor review ID record review with key {old_business_id},{review_id}")
            else:
                print(f"DRY_RUN:, would have deleted review with key {old_business_id},{review_id}")
            continue

        if "createdAt" not in review:
            print(f"WARN: Review {old_business_id}, {review_id} has no createdAt attribute. Skipping.")
            continue
        if not is_legacy_business_id(old_business_id):
            print(f"WARN: Review {old_business_id}, {review_id} has invalid business ID. Skipping.")
            continue

        new_business_id = get_new_business_id(old_business_id)
        review['userId']['S'] = new_business_id

        if not dry_run:
            # use transaction to ensure atomicity
            dynamodb.transact_write_items(
                TransactItems=[
                    {
                        'Put': {
                            'TableName': 'Review',
                            'Item': review
                        }
                    },
                    {
                        'Delete': {
                            'TableName': 'Review',
                            'Key': {'userId': {'S': old_business_id}, 'uniqueId': {'S': review_id}}
                        }
                    },
                ]
            )
            print(f"Updated review with key {old_business_id},{review_id} with {new_business_id},{review_id}")
        else:
            print(
                f"DRY_RUN:, would have deleted review with key {old_business_id},{review_id} and created new review item with key {new_business_id},{review_id}")


def main(dry_run=False):
    update_review_table(dry_run)

if __name__ == "__main__":
    if __name__ == "__main__":
        parser = argparse.ArgumentParser(description='Shorten business IDs in review table')
    parser.add_argument('--dry-run', action='store_true', help='Print changes without modifying the database')
    args = parser.parse_args()

    main(dry_run=args.dry_run)
