unset AWS_REGION
export AWS_PARTITION="aws"
export CLUSTER_NAME="<your-cluster-name>"
export AWS_DEFAULT_REGION="<your-region>"
export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export KARPENTER_NAMESPACE="kube-system"

SUBNETS=()
while IFS= read -r subnet_id; do
  SUBNETS+=("$subnet_id")
done < <(aws eks describe-cluster --name "${CLUSTER_NAME}" \
  --query 'cluster.resourcesVpcConfig.subnetIds' --output text | tr '\t' '\n')
CLUSTER_SG=$(aws eks describe-cluster --name "${CLUSTER_NAME}" \
  --query 'cluster.resourcesVpcConfig.clusterSecurityGroupId' --output text)
