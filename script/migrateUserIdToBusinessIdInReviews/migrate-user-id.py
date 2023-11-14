import argparse
from typing import List

import boto3
from boto3.dynamodb.conditions import Key

dynamodb = boto3.client('dynamodb')


def get_users_with_active_business_id() -> List[dict]:
    response = dynamodb.scan(
        TableName='User',
    )

    # count and filter the users with activeBusinessId
    users = [item for item in response['Items'] if item.get('activeBusinessId', {}).get('S', '') != '']

    print("There are a total of " + str(len(users)) + " users with activeBusinessId, out of " + str(
        len(response['Items'])) + " users in total.")
    print(f"These users are {[user['userId']['S'] for user in users]}")

    return users


def get_all_legacy_reviews_for_user(user_id: str) -> List[dict]:
    """
    Legacy reviews are defined as reviews with userId as its partition key, and uniqueId as its sort key.

    :param user_id:
    :return: list of review objects (excluding their unique vendor review ID records)
    """
    response = dynamodb.query(
        TableName='Review',
        KeyConditionExpression='userId = :userId',
        ExpressionAttributeValues={
            ':userId': {'S': user_id}
        }
    )

    reviews = [item for item in response['Items'] if
               not item.get('uniqueId', '').get('S', '').startswith('#UNIQUE_VENDOR_REVIEW_ID#')]

    print("There are a total of " + str(len(reviews)) + " legacy reviews for user " + user_id)

    return reviews


def main(dry_run=False):
    users = get_users_with_active_business_id()

    # for each user, get all reviews
    for user in users:
        user_id = user['userId']['S']
        business_id = user['activeBusinessId']['S']
        reviews = get_all_legacy_reviews_for_user(user['userId']['S'])

        for review in reviews:
            # for each review, update the userId to businessId
            review['userId']['S'] = business_id

            # prepare unique vendor review id records
            unique_vendor_review_id_record_sort_key = f"#UNIQUE_VENDOR_REVIEW_ID#{review['vendorReviewId']['S']}"

            if dry_run:
                print("\n")
                print(f"Would have processed review with ID {review['uniqueId']['S']} and user ID {user_id} by:")
                print(
                    f"1. Add new review with partition key {business_id} and sort key {review['uniqueId']['S']}")
                print(f"3. Delete old review with partition key {user_id} and sort key {review['uniqueId']['S']}")
                print(
                    f"4. Delete old unique vendor review ID record with partition key {user_id} and sort key {unique_vendor_review_id_record_sort_key}")
                print("\n")

            else:
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
                                'Key': {'userId': {'S': user_id}, 'uniqueId': {'S': review['uniqueId']['S']}}
                            }
                        },
                        {
                            'Delete': {
                                'TableName': 'Review',
                                'Key': {'userId': {'S': user_id},
                                        'uniqueId': {'S': unique_vendor_review_id_record_sort_key}}
                            }
                        }
                    ]
                )

                print(f"Successfully processed review with ID {review['uniqueId']['S']} and business ID {business_id}")


if __name__ == "__main__":
    if __name__ == "__main__":
        parser = argparse.ArgumentParser(description='Convert userId to businessId as partition key of reviews.')
    parser.add_argument('--dry-run', action='store_true', help='Print changes without modifying the database')
    args = parser.parse_args()

    main(dry_run=args.dry_run)
