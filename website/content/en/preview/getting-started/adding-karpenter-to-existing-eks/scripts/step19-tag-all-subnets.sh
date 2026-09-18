SUBNETS=()
while IFS= read -r subnet_id; do
  SUBNETS+=("$subnet_id")
done < <(aws eks describe-cluster --name "${CLUSTER_NAME}" \
  --query 'cluster.resourcesVpcConfig.subnetIds' --output text | tr '\t' '\n')
