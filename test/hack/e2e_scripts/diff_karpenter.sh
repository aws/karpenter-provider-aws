CHART="oci://$ECR_ACCOUNT_ID.dkr.ecr.$ECR_REGION.amazonaws.com/karpenter/snapshot/karpenter"
if [[ "$PRIVATE_CLUSTER" == "true" ]]; then
  CHART="oci://$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/karpenter/snapshot/karpenter"
fi

helm diff upgrade --namespace kube-system \
karpenter "${CHART}" \
--version 0-$(git rev-parse HEAD) \
--reuse-values --three-way-merge --detailed-exitcode --no-hooks
