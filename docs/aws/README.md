
# Set Up Permissions for [AWS-SDK-For-Go](https://docs.aws.amazon.com/sdk-for-go/)
Your workloads must be configured with AWS Credentials to read or modify AWS resources in your account. We recommend using IRSA (IAM Roles for Service Accounts) to manage these permissions.

## Add Custom Policies
```
aws iam create-policy --policy-name KarpenterSQS --policy-document file://docs/aws/iam/sqs-iam-policy.json
```

## Create IRSA
* You can apply multiple policies using `--attach-policy-arn "policyArn1,policyArn2,policyArn3"`
```
CLUSTER=<your_cluster>
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
eksctl create iamserviceaccount --cluster ${CLUSTER} \
--name default \
--namespace karpenter \
--attach-policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/KarpenterSQS" \
--override-existing-serviceaccounts \
--approve
```
