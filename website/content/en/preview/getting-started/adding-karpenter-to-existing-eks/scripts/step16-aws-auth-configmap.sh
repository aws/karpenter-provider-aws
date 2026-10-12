NODE_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME}"

if ! eksctl get iamidentitymapping --cluster "${CLUSTER_NAME}" --arn "${NODE_ROLE_ARN}" 2>/dev/null | grep -q .; then
  eksctl create iamidentitymapping \
    --cluster "${CLUSTER_NAME}" \
    --arn "${NODE_ROLE_ARN}" \
    --username "system:node:{{EC2PrivateDNSName}}" \
    --group system:bootstrappers \
    --group system:nodes
fi
