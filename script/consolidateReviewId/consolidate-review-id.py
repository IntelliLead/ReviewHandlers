import argparse

import boto3
from boto3.dynamodb.conditions import Key

dynamodb = boto3.client('dynamodb')


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
    response = dynamodb.query(
        TableName='Review',
        KeyConditionExpression='userId = :userId',
        ExpressionAttributeValues={
            ':userId': {'S': user_id}
        }
    )

    reviews_dict = [item for item in response['Items'] if
                    not item.get('uniqueId', '').get('S', '').startswith('#UNIQUE_VENDOR_REVIEW_ID#')]

    reviews_dict = sorted(reviews_dict, key=lambda x: int(x['uniqueId']['S']))

    # debug
    print("There are a total of " + str(len(reviews_dict)) + " reviews for user " + user_id + ". The review IDs are:")
    for review in reviews_dict:
        for key, value in review.items():
            if key == 'uniqueId':
                print(value['S'])

    return reviews_dict


def get_all_user_ids():
    response = dynamodb.scan(
        TableName='User',
        ProjectionExpression='userId')

    return [item['userId']['S'] for item in response['Items']]


def main(dry_run=False):
    user_ids = get_all_user_ids()

    for user_id in user_ids:
        reviews = get_all_review_objects_for_user(user_id)

        # THE COMMENTED OUT ASSUMPTION IS INCORRECT. SOME USERS HAVE IT MESSED UP FROM SINGLE CHAR FOR SOME REASON
        # if len(reviews) <= 62:  # If only the special records and <= 62 normal records, skip processing for this user
        #     print(f"Skipping user {user_id} because there are only {len(reviews)} reviews for this user. It's review "
        #           f"IDs are correct")
        #     continue

        next_review_id = "048"

        for old_review in reviews:
            # debug
            print("Processing review " + str(old_review) + "\n")

            old_review_id = old_review['uniqueId']['S']

            if old_review_id == next_review_id:
                print(f"Skipping review ID {old_review_id} for user {user_id} because it is already correct")
                next_review_id = get_next_review_id(next_review_id)  # Move to the next ID
                continue

            if dry_run:
                print(f"Would change review ID from {old_review_id} to {next_review_id} for user {user_id}")
                print(f"Would invoke put_item with {old_review}")
                print(f"Would invoke delete_item with key 'userId': {user_id}, 'reviewId': {old_review_id}")
            else:
                old_review['uniqueId']['S'] = next_review_id
                dynamodb.put_item(
                    TableName='Review',
                    Item=old_review)  # This writes the review with the new ID and overwrites if it already exists
                dynamodb.delete_item(
                    TableName='Review',
                    Key={'userId': {'S': user_id}, 'uniqueId': {'S': old_review_id}})
                print(f"Successfully change review ID from {old_review_id} to {next_review_id} for user {user_id}")

            next_review_id = get_next_review_id(next_review_id)  # Move to the next ID


if __name__ == "__main__":
    if __name__ == "__main__":
        parser = argparse.ArgumentParser(description='Make review IDs sequential.')
    parser.add_argument('--dry-run', action='store_true', help='Print changes without modifying the database')
    args = parser.parse_args()

    main(dry_run=args.dry_run)
