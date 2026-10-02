aws ec2 describe-subnets \
  --filters "Name=tag:karpenter.sh/discovery,Values=${CLUSTER_NAME}" \
  --query 'Subnets[].{id:SubnetId,az:AvailabilityZone}' --output table

aws ec2 describe-security-groups \
  --filters "Name=tag:karpenter.sh/discovery,Values=${CLUSTER_NAME}" \
  --query 'SecurityGroups[].{id:GroupId,name:GroupName}' --output table
