#!/usr/bin/env bash
set -euo pipefail

export AWS_REGION=us-east-1
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity | jq -r '.Account')
MESSAGE="{\"releaseType\":\"periodic\",\"releaseIdentifier\":\"HEAD\",\"prNumber\":\"none\",\"githubAccount\":\"aws\"}"
QUEUE_URL=https://sqs.us-east-1.amazonaws.com/${AWS_ACCOUNT_ID}/ReleaseQueue
aws sqs send-message --queue-url $QUEUE_URL --message-body $MESSAGE