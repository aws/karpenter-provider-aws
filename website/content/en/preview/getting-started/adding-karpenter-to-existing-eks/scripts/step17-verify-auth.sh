# For API / API_AND_CONFIG_MAP:
aws eks list-access-entries --cluster-name "${CLUSTER_NAME}" \
  --query "accessEntries[?contains(@, 'KarpenterNodeRole')]"

# For CONFIG_MAP:
kubectl get configmap aws-auth -n kube-system -o yaml | grep -A2 "KarpenterNodeRole-${CLUSTER_NAME}"
