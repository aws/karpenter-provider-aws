export CLUSTER_NAME="${USER}-karpenter-demo"
export CLUSTER_NAME_SHA=$(echo -n "${CLUSTER_NAME}" | tr -d '"' | sha256sum | cut -c -20)
export AWS_DEFAULT_REGION="us-west-2"
export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
