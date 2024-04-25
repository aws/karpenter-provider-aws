eksctl create iamserviceaccount \
  --cluster "${CLUSTER_NAME}" --name karpenter --namespace "${KARPENTER_NAMESPACE}" \
  --role-name "${CLUSTER_NAME}-karpenter" \
  --attach-policy-arn "arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:policy/KarpenterControllerPolicy-${CLUSTER_NAME}" \
  --role-only \
  --approve

export KARPENTER_IAM_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
