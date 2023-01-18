helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version ${KARPENTER_VERSION} --namespace karpenter --create-namespace \
    --set settings.aws.defaultInstanceProfile="${IAM_INSTANCE_PROFILE_NAME}" \
    --set settings.aws.clusterEndpoint="${CLUSTER_ENDPOINT}" \
    --set settings.aws.clusterName=${CLUSTER_NAME} \
    --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterControllerRole-${CLUSTER_NAME}" \
    --version ${KARPENTER_VERSION} > karpenter.yaml
