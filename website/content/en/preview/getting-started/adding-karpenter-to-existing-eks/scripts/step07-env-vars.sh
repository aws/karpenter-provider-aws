export KARPENTER_NAMESPACE="kube-system"
export KARPENTER_VERSION="{{< param "latest_release_version" >}}"
export AWS_PARTITION="aws"                     # <aws | aws-cn | aws-us-gov>
export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export TEMPOUT="$(mktemp)"
