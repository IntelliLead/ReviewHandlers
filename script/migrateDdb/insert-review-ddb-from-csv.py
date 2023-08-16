import csv
import boto3


def convert_csv_row_to_review_ddb_item(row):
    item = {}
    for key, value in row.items():
        if value != '':
            if key == 'userId' or key == 'uniqueId' or key == 'reply' or key == 'review' or key == 'reviewerName' or key == 'reviewerProfilePhoto' or key == 'vendorEventId' or key == 'vendorReviewId' or key == 'zapierReplyWebhook':
                item[key] = {'S': value}
            else:
                item[key] = {'N': value}
    return item


def insert_ddb_item(table_name_, item):
    dynamodb = boto3.client('dynamodb')

    # debug
    print("Inserting item: ", item)

    response = dynamodb.put_item(TableName=table_name_, Item=item)
    return response


def process_csv_file(csv_file, table_name_):
    with open(csv_file, 'r') as file:
        reader = csv.DictReader(file)
        for row in reader:
            item = convert_csv_row_to_review_ddb_item(row)
            response = insert_ddb_item(table_name_, item)
            print(f"Inserted item: {item}")
            print(f"Response: {response}")
            print()


# Provide the path to your CSV file and the DynamoDB table name
csv_file_path = 'data/reviews.csv'
table_name = 'Review'

process_csv_file(csv_file_path, table_name)
