export CLUSTER_NAME="${CLUSTER_NAME:-karpenter-test-infrastructure}"
export AWS_PROFILE="${AWS_PROFILE:-karpenter-ci}"
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
export AWS_REGION="${AWS_REGION:-us-west-2}"
export AWS_PAGER=""
