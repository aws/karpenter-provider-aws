
# Setup Permissions
Karpenter's pod requires AWS Credentials to read or modify AWS resources in your account. We recommend using IRSA (IAM Roles for Service Accounts) to manage these permissions.

```
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
CLUSTER_NAME=<your_cluster>
```

## Create the Karpenter IAM Policy
This command will create an IAM Policy with access to all of the resources for all of Karpenter's features. For increased security, you may wish to reduce the permissions according to your use case.
```
aws iam create-policy --policy-name Karpenter --policy-document "$(cat <<-EOM
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": [
                "eks:DescribeNodegroup",
                "eks:UpdateNodegroupConfig"
            ],
            "Effect": "Allow",
            "Resource": "*"
        },
        {
            "Action": [
                "autoscaling:DescribeAutoScalingGroups",
                "autoscaling:UpdateAutoScalingGroup"
            ],
            "Effect": "Allow",
            "Resource": "*"
        },
        {
            "Action": [
                "sqs:GetQueueAttributes",
                "sqs:GetQueueUrl"
            ],
            "Effect": "Allow",
            "Resource": "*"
        }
    ]
}
EOM
)"
```

## Create using IAM Roles for Service Accounts (IRSA)
This command will associate the AWS IAM Policy you created above with the Kubernetes Service Account used by Karpenter.
```
eksctl create iamserviceaccount --cluster ${CLUSTER_NAME} \
--name default \
--namespace karpenter \
--attach-policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/Karpenter" \
--override-existing-serviceaccounts \
--approve
```

## Verify the Permissions
You should see an annotation with key eks.amazonaws.com/role-arn
```
kubectl get serviceaccount default -n karpenter -ojsonpath="{.metadata.annotations}"
```

## Cleanup
```
eksctl delete iamserviceaccount --cluster ${CLUSTER_NAME} --name default --namespace karpenter
aws iam delete-policy --policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/Karpenter
```
