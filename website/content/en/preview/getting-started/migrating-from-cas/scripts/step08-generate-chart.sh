helm template --namespace karpenter \
    karpenter karpenter/karpenter \
    --set aws.defaultInstanceProfile=KarpenterInstanceNodeRole \
    --set clusterEndpoint="${OIDC_ENDPOINT}" \
    --set clusterName=${CLUSTER_NAME} \
    --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterController" \
    --version ${KARPENTER_VERSION} > karpenter.yaml
