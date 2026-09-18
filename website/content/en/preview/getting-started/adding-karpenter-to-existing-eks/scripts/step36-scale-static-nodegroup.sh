eksctl scale nodegroup \
  --cluster "${CLUSTER_NAME}" \
  --name <static-nodegroup-name> \
  --nodes <new-lower-count> \
  --nodes-min <new-lower-count>
