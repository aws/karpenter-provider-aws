# Logout of helm registry to perform an unauthenticated pull against the public ECR
helm registry logout public.ecr.aws

# Set AWS_REGION env variable and annotate Karpenter service account
# with the IAM Role ARN when using Fargate profile
helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version "${KARPENTER_VERSION}" --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
  --set "settings.clusterName=${CLUSTER_NAME}" \
  --set "settings.interruptionQueue=${CLUSTER_NAME}" \
  --set controller.resources.requests.cpu=1 \
  --set controller.resources.requests.memory=1Gi \
  --set controller.resources.limits.cpu=1 \
  --set controller.resources.limits.memory=1Gi \
  #  --set controller.env[0].name=AWS_REGION \
  #  --set controller.env[0].value=${AWS_DEFAULT_REGION} \
  #  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter" \
  --wait
