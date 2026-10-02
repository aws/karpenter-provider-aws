helm registry logout public.ecr.aws 2>/dev/null || true

# Same install as the with-queue variant but with `settings.interruptionQueue`
# omitted — the controller runs without an SQS queue and will not handle
# spot interruptions, rebalance recommendations, or scheduled maintenance
# events gracefully.
helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter \
  --version "${KARPENTER_VERSION}" \
  --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
  --set "settings.clusterName=${CLUSTER_NAME}" \
  --set "serviceAccount.annotations.eks\.amazonaws\.com/role-arn=${KARPENTER_IAM_ROLE_ARN}" \
  --set controller.resources.requests.cpu=1 \
  --set controller.resources.requests.memory=1Gi \
  --set controller.resources.limits.cpu=1 \
  --set controller.resources.limits.memory=1Gi \
  --set "nodeSelector.<your-system-label-key>=<your-system-label-value>" \
  --wait
