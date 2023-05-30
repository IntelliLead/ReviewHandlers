#!/usr/bin/env bash

# 0. check if DEV_ACCOUNT is present
if [[ -z "${DEV_ACCOUNT}" ]]; then
    echo >&2 "DEV_ACCOUNT is not set!"
    kill -INT $$ # kill the current process instead of exiting shell
else
    echo "DEV_ACCOUNT is set to $DEV_ACCOUNT"
fi

# 1. refresh SSO access token with aws sso login
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY
unset AWS_SESSION_TOKEN

eval "$(aws2-wrap --profile AdministratorAccess-"$DEV_ACCOUNT" --export)"

echo "Exported env vars to assume AWS profile AdministratorAccess-$DEV_ACCOUNT \nAWS_ACCESS_KEY_ID: ${AWS_ACCESS_KEY_ID} \nAWS_SECRET_ACCESS_KEY: ${AWS_SECRET_ACCESS_KEY} \nAWS_SESSION_TOKEN: ${AWS_SESSION_TOKEN} "

# TODO: get non-zero exit status. Following code does not work
#if [ $? -ne 0 ]; then
#    echo "aws2-wrap exited with non-zero code. Is it installed? Did you do 'aws configure sso/login' first?" >&2
#else
#  echo "Successfully updated AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, and AWS_SESSION_TOKEN to assume AWS profile AdministratorAccess-$DEV_ACCOUNT"
#fi
