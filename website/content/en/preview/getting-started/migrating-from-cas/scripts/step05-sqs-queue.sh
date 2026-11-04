cat << EOF > sqs-queue-policy.json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": [
                    "events.amazonaws.com",
                    "sqs.amazonaws.com"
                ]
            },
            "Action": "sqs:SendMessage",
            "Resource": "arn:${AWS_PARTITION}:sqs:${AWS_REGION}:${AWS_ACCOUNT_ID}:${CLUSTER_NAME}",
            "Condition": {
                "ArnEquals": {
                    "aws:SourceArn": [
                        "arn:${AWS_PARTITION}:events:${AWS_REGION}:${AWS_ACCOUNT_ID}:rule/KarpenterInterruptionRule-${CLUSTER_NAME}-*"
                    ]
                }
            }
        }
    ]
}
EOF

aws sqs create-queue --queue-name "${CLUSTER_NAME}" --attributes '{"MessageRetentionPeriod":"300","SqsManagedSseEnabled":"true"}'

QUEUE_ARN=$(aws sqs get-queue-attributes --queue-url "$(aws sqs get-queue-url --queue-name "${CLUSTER_NAME}" --query 'QueueUrl' --output text)" --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)

aws sqs set-queue-attributes --queue-url "$(aws sqs get-queue-url --queue-name "${CLUSTER_NAME}" --query 'QueueUrl' --output text)" \
    --attributes "{\"Policy\":$(cat sqs-queue-policy.json | jq -c '.' | jq -Rs '.')}"
