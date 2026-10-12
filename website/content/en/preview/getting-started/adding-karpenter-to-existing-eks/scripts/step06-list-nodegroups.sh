aws eks list-nodegroups --cluster-name "${CLUSTER_NAME}"
aws eks describe-nodegroup --cluster-name "${CLUSTER_NAME}" \
  --nodegroup-name <each-nodegroup> \
  --query 'nodegroup.{name:nodegroupName,size:scalingConfig,labels:labels,taints:taints}'
