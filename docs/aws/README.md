
# AWS
## Installation
Karpenter's pod requires AWS Credentials to read or modify AWS resources in your account. We recommend using IRSA (IAM Roles for Service Accounts) to manage these permissions.

```bash
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
CLUSTER_NAME=<your_cluster>
REGION=<your_region>
```

### Create Karpenter IAM Resources
This command will create IAM resources used by Karpenter. For production use, please review and restrict these permissions as necessary.
```bash
// TODO, point to github raw uri
aws cloudformation deploy \
  --stack-name Karpenter \
  --template-file ./docs/aws/karpenter.cloudformation.yaml \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides OpenIDConnectIdentityProvider=$(aws eks describe-cluster --name ${CLUSTER_NAME} | jq -r ".cluster.identity.oidc.issuer" | cut -c9-)
```

### Enable IRSA
Enables IRSA for your cluster. This command is idempotent, but only needs to be executed once per cluster.
```bash
eksctl utils associate-iam-oidc-provider \
--region ${REGION} \
--cluster ${CLUSTER_NAME} \
--approve
```

### Attach the Permissions
```bash
kubectl patch serviceaccount karpenter -n karpenter --patch "$(cat <<-EOM
metadata:
  annotations:
     eks.amazonaws.com/role-arn: arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterControllerRole
EOM
)"
```
If you've already installed the Karpenter controller, you'll need to restart the pod to load the credentials.
```bash
kubectl delete pods -n karpenter -l control-plane=karpenter
```

### Cleanup
```bash
aws cloudformation delete-stack --stack-name Karpenter
aws ec2 describe-launch-templates | jq -r ".LaunchTemplates[].LaunchTemplateName" | grep Karpenter | xargs -I{} aws ec2 delete-launch-template --launch-template-name {}
```
