import argparse

import boto3
from boto3.dynamodb.conditions import Key

dynamodb = boto3.resource('dynamodb')
review_table = dynamodb.Table('Review')  # Replace 'YourTableName' with your table name
user_table = dynamodb.Table('User')  # Replace with your user table name


def get_next_review_id(review_id):
    numeric_str = review_id
    new_numeric_str = increment_ascii_codes(numeric_str, len(numeric_str) // 3 - 1)

    # debug
    print("The next review ID after " + review_id + " is " + new_numeric_str + "\n")

    return new_numeric_str


def increment_ascii_codes(numeric_str, idx):
    if idx < 0:
        return f"{48:03}" + numeric_str

    ascii_code = int(numeric_str[idx * 3:idx * 3 + 3])
    new_ascii_code, carry = get_next_ascii_code(ascii_code)
    if carry:
        new_numeric_str = increment_ascii_codes(numeric_str[:idx * 3], idx - 1)
        return new_numeric_str + f"{new_ascii_code:03}"
    return numeric_str[:idx * 3] + f"{new_ascii_code:03}" + numeric_str[idx * 3 + 3:]


def get_next_ascii_code(last_ascii_code):
    if last_ascii_code == 57:
        return 65, False
    elif last_ascii_code == 90:
        return 97, False
    elif last_ascii_code == 122:
        return 48, True
    else:
        return last_ascii_code + 1, False


def get_all_review_objects_for_user(user_id):
    reviews = []

    # Start the query
    response = review_table.query(KeyConditionExpression=Key('userId').eq(user_id))
    reviews.extend([item for item in response['Items'] if not item.get('uniqueId', '').startswith('#UNIQUE_VENDOR_REVIEW_ID#')])

    # Handle pagination
    while 'LastEvaluatedKey' in response:
        response = review_table.query(
            KeyConditionExpression=Key('userId').eq(user_id),
            ExclusiveStartKey=response['LastEvaluatedKey']
        )
        reviews.extend([item for item in response['Items'] if not item.get('uniqueId', '').startswith('#UNIQUE_VENDOR_REVIEW_ID#')])

    # debug
    print("The reviews for user " + user_id + " are " + str(reviews) + "\n")
    return reviews


def get_all_user_ids():
    user_ids = []
    response = user_table.scan(ProjectionExpression='userId')

    while True:
        user_ids.extend([item['userId'] for item in response['Items']])
        # If there are more items to be fetched, fetch them
        if 'LastEvaluatedKey' in response:
            response = user_table.scan(ProjectionExpression='userId',
                                       ExclusiveStartKey=response['LastEvaluatedKey'])
        else:
            break
    return user_ids


def main(dry_run=False):
    user_ids = get_all_user_ids()

    for user_id in user_ids:
        reviews = get_all_review_objects_for_user(user_id)

        if len(reviews) <= 62:  # If only the special records and <= 62 normal records, skip processing for this user
            continue

        next_review_id = get_next_review_id(reviews[61]['uniqueId'])  # Start after the 62nd one

        for old_review in reviews[62:]:
            # debug
            print("Processing review " + str(old_review) + "\n")

            old_review_id = old_review['uniqueId']
            old_review['uniqueId'] = next_review_id

            if dry_run:
                print(f"Would change review ID from {old_review_id} to {next_review_id} for user {user_id}")
            else:
                review_table.put_item(Item=old_review)  # This writes the review with the new ID and overwrites if it already exists
                review_table.delete_item(Key={'userId': user_id, 'reviewId': old_review_id})
                print(f"Successfully change review ID from {old_review_id} to {next_review_id} for user {user_id}")

            next_review_id = get_next_review_id(next_review_id)  # Move to the next ID


if __name__ == "__main__":
    if __name__ == "__main__":
        parser = argparse.ArgumentParser(description='Make review IDs sequential.')
    parser.add_argument('--dry-run', action='store_true', help='Print changes without modifying the database')
    args = parser.parse_args()

    main(dry_run=args.dry_run)

