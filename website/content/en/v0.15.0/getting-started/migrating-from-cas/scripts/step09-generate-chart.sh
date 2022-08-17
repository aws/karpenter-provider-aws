helm template --namespace karpenter \
    karpenter karpenter/karpenter \
    --set aws.defaultInstanceProfile=KarpenterInstanceProfile \
    --set clusterEndpoint="${CLUSTER_ENDPOINT}" \
    --set clusterName=${CLUSTER_NAME} \
    --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterControllerRole-${CLUSTER_NAME}" \
    --version ${KARPENTER_VERSION} > karpenter.yaml
