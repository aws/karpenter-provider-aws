export CLUSTER_NAME="${CLUSTER_NAME:-karpenter-test-cluster}"
export AWS_PROFILE="${AWS_PROFILE:-karpenter-ci}"
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
export AWS_REGION="${AWS_REGION:-us-west-2}"
export KARPENTER_VERSION="${KARPENTER_VERSION:-v0.9.0}"
export AWS_PAGER=""
