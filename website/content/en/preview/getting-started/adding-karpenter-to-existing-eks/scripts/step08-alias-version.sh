export K8S_VERSION="$(aws eks describe-cluster --name "${CLUSTER_NAME}" \
  --query 'cluster.version' --output text)"

export ALIAS_VERSION="$(aws ec2 describe-images --query 'Images[0].Name' \
  --image-ids "$(aws ssm get-parameter \
    --name "/aws/service/eks/optimized-ami/${K8S_VERSION}/amazon-linux-2023/x86_64/standard/recommended/image_id" \
    --query Parameter.Value --output text)" \
  --output text | sed -E 's/.*(v[[:digit:]]+).*/\1/')"

echo "$ALIAS_VERSION"
