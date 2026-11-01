NODE_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME}"

if ! aws eks describe-access-entry \
       --cluster-name "${CLUSTER_NAME}" \
       --principal-arn "${NODE_ROLE_ARN}" >/dev/null 2>&1; then
  aws eks create-access-entry \
    --cluster-name "${CLUSTER_NAME}" \
    --principal-arn "${NODE_ROLE_ARN}" \
    --type EC2_LINUX
fi
