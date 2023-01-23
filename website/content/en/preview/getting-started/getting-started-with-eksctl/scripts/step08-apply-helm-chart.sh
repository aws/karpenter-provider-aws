docker logout public.ecr.aws
helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version ${KARPENTER_VERSION} --namespace karpenter --create-namespace \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
  --set karpenter-core.serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
  --set karpenter-core.settings.aws.clusterName=${CLUSTER_NAME} \
  --set karpenter-core.settings.aws.clusterEndpoint=${CLUSTER_ENDPOINT} \
  --set karpenter-core.settings.aws.defaultInstanceProfile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
  --set karpenter-core.settings.aws.interruptionQueueName=${CLUSTER_NAME} \
  --wait
