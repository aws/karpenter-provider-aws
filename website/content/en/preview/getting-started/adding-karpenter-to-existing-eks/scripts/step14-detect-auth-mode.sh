aws eks describe-cluster --name "${CLUSTER_NAME}" \
  --query 'cluster.accessConfig.authenticationMode' --output text
