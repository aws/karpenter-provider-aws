
# AWS
## Installation
Karpenter's pod requires AWS Credentials to read or modify AWS resources in your account. We recommend using IRSA (IAM Roles for Service Accounts) to manage these permissions.

```
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
CLUSTER_NAME=<your_cluster>
REGION=<your_region>
```

### Create the Karpenter IAM Policy
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
    },
    {
      "Action": [
        "ec2:DescribeLaunchTemplates",
        "ec2:CreateLaunchTemplate",
        "ec2:CreateFleet",
        "ec2:RunInstances",
        "ec2:CreateTags",
        "ec2:DescribeSubnets",
        "eks:DescribeCluster",
        "iam:GetRole",
        "iam:CreateRole",
        "iam:AddRoleToInstanceProfile",
        "iam:PassRole",
        "iam:GetInstanceProfile",
        "iam:CreateInstanceProfile",
        "iam:AttachRolePolicy"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOM
)"
```

### Associate the IAM Role with your Kubernetes Service Account
These commands will associate the AWS IAM Policy you created above with the Kubernetes Service Account used by Karpenter.
```
eksctl utils associate-iam-oidc-provider \
--region ${REGION} \
--cluster ${CLUSTER_NAME} \
--approve

eksctl create iamserviceaccount --cluster ${CLUSTER_NAME} \
--name default \
--namespace karpenter \
--attach-policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/Karpenter" \
--override-existing-serviceaccounts \
--approve
```

### Verify the Permissions
You should see an annotation with key eks.amazonaws.com/role-arn
```
kubectl get serviceaccount default -n karpenter -ojsonpath="{.metadata.annotations}"
```
If you've already installed the Karpenter controller, you'll need to restart the pod to load the credentials.
```
kubectl delete pods -n karpenter -l control-plane=karpenter
```

### Cleanup
```bash
eksctl delete iamserviceaccount --cluster ${CLUSTER_NAME} --name default --namespace karpenter
aws iam delete-policy --policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/Karpenter

# Remove Karpenter generated resources
aws iam remove-role-from-instance-profile --instance-profile-name KarpenterNodeRole --role-name KarpenterNodeRole
aws iam delete-instance-profile --instance-profile-name KarpenterNodeRole
aws iam detach-role-policy --role-name KarpenterNodeRole --policy-arn arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore
aws iam detach-role-policy --role-name KarpenterNodeRole --policy-arn arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy
aws iam detach-role-policy --role-name KarpenterNodeRole --policy-arn arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy
aws iam detach-role-policy --role-name KarpenterNodeRole --policy-arn arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly
aws iam delete-role --role-name KarpenterNodeRole
aws ec2 delete-launch-template --launch-template-name KarpenterLaunchTemplate
```
