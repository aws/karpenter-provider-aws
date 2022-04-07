helm template --namespace karpenter \
    karpenter karpenter/karpenter \
    --set aws.defaultInstanceProfile=KarpenterInstanceNodeRole \
    --set clusterEndpoint="${OIDC_ENDPOINT}" \
    --set clusterName=${CLUSTER_NAME} \
    --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::${ACCOUNT_NAME}:role/KarpenterController" \
    --version {{< param "latest_release_version" >}} > karpenter.yaml