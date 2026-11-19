# Get the instance IDs of nodes matching your system label, then
# resolve each to its subnet. Adjust the label to match your cluster.
INSTANCE_IDS=()
while IFS= read -r provider_id; do
  [ -n "$provider_id" ] && INSTANCE_IDS+=("${provider_id##*/}")
done < <(kubectl get nodes -l <your-system-label-key>=<your-system-label-value> \
  -o jsonpath='{range .items[*]}{.spec.providerID}{"\n"}{end}')

SUBNETS=()
while IFS= read -r subnet_id; do
  [ -n "$subnet_id" ] && SUBNETS+=("$subnet_id")
done < <(aws ec2 describe-instances --instance-ids "${INSTANCE_IDS[@]}" \
  --query 'Reservations[].Instances[].SubnetId' --output text | tr '\t' '\n' | sort -u)

echo "Discovered ${#SUBNETS[@]} routable subnet(s) from system nodes:"
printf '  %s\n' "${SUBNETS[@]}"

aws ec2 create-tags \
  --resources "${SUBNETS[@]}" \
  --tags "Key=karpenter.sh/discovery,Value=${CLUSTER_NAME}"

CLUSTER_SG=$(aws eks describe-cluster --name "${CLUSTER_NAME}" \
  --query 'cluster.resourcesVpcConfig.clusterSecurityGroupId' --output text)

aws ec2 create-tags \
  --resources "${CLUSTER_SG}" \
  --tags "Key=karpenter.sh/discovery,Value=${CLUSTER_NAME}"
