export AWS_PARTITION="aws" # if you are not using standard partitions, you may need to configure to aws-cn / aws-us-gov
export AWS_REGION="$(aws configure list | grep region | tr -s " " | cut -d" " -f3)"
export OIDC_ENDPOINT="$(aws eks describe-cluster --name "${CLUSTER_NAME}" \
    --query "cluster.identity.oidc.issuer" --output text)"
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query 'Account' \
    --output text)
export K8S_VERSION=1.30
export ALIAS_VERSION=$(aws ssm get-parameter --name "/aws/service/eks/optimized-ami/${K8S_VERSION}/amazon-linux-2023/x86_64/standard/recommended/image_id" --query Parameter.Value | xargs aws ec2 describe-images --query 'Images[0].Name' --image-ids | sed -r 's/^.*(v[[:digit:]]+).*$/\1/')
