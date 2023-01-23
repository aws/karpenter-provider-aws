helm template karpenter oci://public.ecr.aws/karpenter/karpenter --version ${KARPENTER_VERSION} --namespace karpenter \
    --set karpenter-core.settings.aws.defaultInstanceProfile=KarpenterInstanceProfile \
    --set karpenter-core.settings.aws.clusterEndpoint="${CLUSTER_ENDPOINT}" \
    --set karpenter-core.settings.aws.clusterName=${CLUSTER_NAME} \
    --set karpenter-core.serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterControllerRole-${CLUSTER_NAME}" \
    --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterControllerRole-${CLUSTER_NAME}" \
    --version ${KARPENTER_VERSION} > karpenter.yaml
