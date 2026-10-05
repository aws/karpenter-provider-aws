aws sqs get-queue-url --queue-name "${CLUSTER_NAME}"
aws iam get-role --role-name "KarpenterNodeRole-${CLUSTER_NAME}" --query Role.Arn
