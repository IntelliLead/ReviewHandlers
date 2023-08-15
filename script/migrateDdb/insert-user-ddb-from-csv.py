import csv
import boto3


def convert_csv_row_to_user_ddb_item(row):
    item = {}

    for key, value in row.items():
        if key == 'userId' or key == 'uniqueId' or key == 'language' or key == 'lineProfilePicture' or key == 'lineUsername' or key == 'quickReplyMessage':
            item[key] = {'S': value}
        elif key == 'autoQuickReplyEnabled' or key == 'subscriptionTier':
            # convert string to boolean
            value = value.lower() == 'true'
            item[key] = {'BOOL': value}
        else:
            item[key] = {'N': value}

    print("Constructed item: " + str(item))
    return item


def insert_ddb_item(table_name_, item):
    dynamodb = boto3.client('dynamodb')

    # debug
    print(item)

    response = dynamodb.put_item(TableName=table_name_, Item=item)
    return response
    # return "done"


def process_csv_file(csv_file, table_name_):
    with open(csv_file, 'r') as file:
        reader = csv.DictReader(file)
        for row in reader:
            item = convert_csv_row_to_user_ddb_item(row)
            response = insert_ddb_item(table_name_, item)
            print(f"Inserted item: {item}")
            print(f"Response: {response}")
            print()


# Provide the path to your CSV file and the DynamoDB table name
csv_file_path = 'data/users.csv'
table_name = 'User'

process_csv_file(csv_file_path, table_name)
