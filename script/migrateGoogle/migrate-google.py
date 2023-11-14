import argparse
from typing import List

import boto3

dynamodb = boto3.client('dynamodb')


def get_all_businesses():
    response = dynamodb.scan(
        TableName='Business')

    return response['Items']


def get_users(user_ids: List[str]):
    users = []
    for userId in user_ids:
        response = dynamodb.get_item(
            TableName='User',
            Key={
                'userId': {'S': userId},
                'uniqueId': {'S': '#'}}
        )
        users.append(response['Item'])

    return users


def main(dry_run):
    businesses = get_all_businesses()

    for business in businesses:
        if "google" in business:
            print(f"Business {business['businessId']['S']} {business['businessName']['S']} has google attributes.")

            google_attribute = business['google']['M']
            users = get_users(business['userIds']['SS'])

            for user in users:
                username = "no line username"
                if "lineUsername" in user:
                    username = user['lineUsername']['S']

                if "google" in user:
                    print(f"User {user['userId']['S']} {username} already has google attribute. Skipping.")
                    continue

                # update user with google attribute
                if not dry_run:
                    dynamodb.update_item(
                        TableName='User',
                        Key={'userId': {'S': user['userId']['S']}, 'uniqueId': {'S': '#'}},
                        UpdateExpression='SET google = :google',
                        ExpressionAttributeValues={':google': {'M': google_attribute}}
                    )
                    print(f"Updated user {user['userId']['S']} {username} with google attribute")
                else:
                    print(f"DRY_RUN: would have updated user {user['userId']['S']} {username} with google attribute")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Update users with businessIds attribute.')
    parser.add_argument('--dry-run', action='store_true', help='Print changes without modifying the database')
    args = parser.parse_args()

    main(dry_run=args.dry_run)
