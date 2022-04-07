---
title: "Migrating from Cluster Autoscaler"
linkTitle: "Migrating from Cluster Autoscaler"
weight: 10
---

This guide will show you how to switch from the [Kubernetes Cluster Autoscaler](https://github.com/kubernetes/autoscaler) to Karpenter for automatic node provisioning.
We will make the following assumptions in this guide

* You will use an existing EKS cluster
* You will use existing VPC and subnets
* You will use existing security groups
* Your nodes are part of one or more node groups
* Your workloads have pod disruption budgets that adhere to [EKS best practices](https://aws.github.io/aws-eks-best-practices/karpenter/)
* Your cluster has an [OIDC provider](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html) for service accounts

This guide will also assume you have the `aws` CLI installed.
You can also perform many of these steps in the console, but we will use the command line for simplicity.

## Create IAM roles

To get started with our migration we first need to create two new IAM roles for nodes provisioned with Karpenter and the Karpenter controller.

To create the Karpenter node role we will use the following policy and commands.

```bash
echo '{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": "ec2.amazonaws.com"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}' > node-trust-policy.json

aws iam create-role --role-name KarpenterInstanceNodeRole \
    --assume-role-policy-document file://node-trust-policy.json
```
Now attach the required policies to the role

```bash
aws iam attach-role-policy --role-name KarpenterInstanceNodeRole \
    --policy-arn arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy

aws iam attach-role-policy --role-name KarpenterInstanceNodeRole \
    --policy-arn arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy

aws iam attach-role-policy --role-name KarpenterInstanceNodeRole \
    --policy-arn arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly

# optional for session manager
aws iam attach-role-policy --role-name KarpenterInstanceNodeRole \
    --policy-arn arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore
```

Now we need to create an IAM role that the Karpenter controller will use to provision new instances.
The controller will be using [IAM Roles for Service Accounts (IRSA)](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) which requires an OIDC endpoint.

If you have another option for using IAM credentials with workloads (e.g. [kube2iam](https://github.com/jtblin/kube2iam)) your steps will be different.

First we need to get the OIDC endpoint for the cluster.
```bash
CLUSTER_NAME=<your existing cluster>
OIDC_ENDPOINT=$(aws eks describe-cluster --name ${CLUSTER_NAME} \
    --query "cluster.identity.oidc.issuer" --output text)
ACCOUNT_NUMBER=$(aws sts get-caller-identity --query 'Account' \
    --output text)
```

Use that information to create our IAM role, inline policy, and trust relationship.
```bash
echo "{
    \"Version\": \"2012-10-17\",
    \"Statement\": [
        {
            \"Effect\": \"Allow\",
            \"Principal\": {
                \"Federated\": \"arn:aws:iam::${ACCOUNT}:oidc-provider/${OIDC_ENDPOINT#*//}\"
            },
            \"Action\": \"sts:AssumeRoleWithWebIdentity\",
            \"Condition\": {
                \"StringEquals\": {
                    \"${OIDC_ENDPOINT#*//}:aud\": \"sts.amazonaws.com\",
                    \"${OIDC_ENDPOINT#*//}:sub\": \"system:serviceaccount:karpenter:karpenter\"
                }
            }
        }
    ]
}" > trust-policy.json

aws iam create-role --role-name KarpenterController \
    --assume-role-policy-document file://trust-policy.json

echo '{
    "Statement": [
        {
            "Action": [
                "ssm:GetParameter",
                "iam:PassRole",
                "ec2:RunInstances",
                "ec2:DescribeSubnets",
                "ec2:DescribeSecurityGroups",
                "ec2:DescribeLaunchTemplates",
                "ec2:DescribeInstances",
                "ec2:DescribeInstanceTypes",
                "ec2:DescribeInstanceTypeOfferings",
                "ec2:DescribeAvailabilityZones",
                "ec2:DeleteLaunchTemplate",
                "ec2:CreateTags",
                "ec2:CreateLaunchTemplate",
                "ec2:CreateFleet"
            ],
            "Effect": "Allow",
            "Resource": "*",
            "Sid": "Karpenter"
        },
        {
            "Action": "ec2:TerminateInstances",
            "Condition": {
                "StringLike": {
                    "ec2:ResourceTag/Name": "*karpenter*"
                }
            },
            "Effect": "Allow",
            "Resource": "*",
            "Sid": "ConditionalEC2Termination"
        }
    ],
    "Version": "2012-10-17"
}' > karpenter-controller-policy.json

aws iam put-role-policy --role-name KarpenterController \
    --policy-name KarpenterController \
    --policy-document file://karpenter-controller-policy.json
```

## Add tags to subnets and security groups

We need to add tags to our nodegroup subnets so Karpenter will know which subnets to use.

```bash
for NODEGROUP in $(aws eks list-nodegroups --cluster-name ${CLUSTER_NAME} \
    --query 'nodegroups' --output text); do aws ec2 create-tags \
        --tags "Key=karpenter.sh/discovery,Value=${CLUSTER_NAME}" \
        --resources $(aws eks describe-nodegroup --cluster-name ${CLUSTER_NAME} \
        --nodegroup-name $NODEGROUP --query 'nodegroup.subnets' --output text )
done
```

Add tags to our node group.
This command only tags the security groups for the first nodegroup in the cluster.
If you have multiple nodegroups or multiple security groups you will need to decide which one Karpenter should use.

```bash
NODEGROUP=$(aws eks list-nodegroups --cluster-name ${CLUSTER_NAME} \
    --query 'nodegroups[0]' --output text)

LAUNCH_TEMPLATE=$(aws eks describe-nodegroup --cluster-name ${CLUSTER_NAME} \
    --nodegroup-name ${NODEGROUP} --query 'nodegroup.launchTemplate.{id:id,version:version}' \
    --output text | tr -s "\t" ",")

SECURITY_GROUPS=$(aws ec2 describe-launch-template-versions \
    --launch-template-id ${LAUNCH_TEMPLATE%,*} --versions ${LAUNCH_TEMPLATE#*,} \
    --query 'LaunchTemplateVersions[0].LaunchTemplateData.SecurityGroupIds' \
    --output text)

aws ec2 create-tags \
    --tags "Key=karpenter.sh/discovery,Value=${CLUSTER_NAME}" \
    --resources ${SECURITY_GROUPS}
```

## Update aws-auth ConfigMap

We need to allow nodes that are using the node IAM role we just created to join the cluter.
To do that we have to modify the `aws-auth` ConfigMap in the cluster.

```
kubectl edit configmap aws-auth -n kube-system
```

You will need to add a section to the mapRoles that looks something like this.
Replace the `${ACCOUNT_NUMBER}` variable with your account, but do not replace the `{{EC2PrivateDNSName}}`.
```
    - groups:
      - system:bootstrappers
      - system:nodes
      rolearn: arn:aws:iam::${ACCOUNT_NUMBER}:role/KarpenterInstanceNodeRole
      username: system:node:{{EC2PrivateDNSName}}
```

The full aws-auth configmap should have two groups.
One for your Karpenter node role and one for your existing node group.

## Deploy Karpenter

We can now generate a full Karpenter deployment yaml from the helm chart.

```bash
KARPENTER_VERSION="v0.8.1"

helm template --namespace karpenter \
    karpenter karpenter/karpenter \
    --set aws.defaultInstanceProfile=KarpenterInstanceNodeRole \
    --set clusterEndpoint="${OIDC_ENDPOINT}" \
    --set clusterName=${CLUSTER_NAME} \
    --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::${ACCOUNT_NAME}:role/KarpenterController" \
    --version ${KARPENTER_VERSION} > karpenter.yaml
```

Modify the following lines in the karpenter.yaml file.

### Set node affinity

Edit the karpenter.yaml file and find the karpenter deployment affinity rules.
Modify the affinity so karpenter will run on one of the existing node group nodes.

The rules should look something like this.
Replace the nodegroup value with your `${NODEGROUP}`
```
      affinity:                      
        nodeAffinity: 
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: karpenter.sh/provisioner-name
                operator: DoesNotExist
            - matchExpressions:
              - key: eks.amazonaws.com/nodegroup
                operator: In
                values:
                - ng-123456
```

Now that our deployment is ready we can create the karpenter namespace, create the provisioner CRD, and then deploy the rest of the karpenter resources.
```bash
kubectl create namespace karpenter
kubectl create -f \
    https://raw.githubusercontent.com/aws/karpenter{{< githubRelRef >}}charts/karpenter/crds/karpenter.sh_provisioners.yaml
kubectl apply -f karpenter.yaml
```

## Create default provisioner

We need to create a default provisioner so Karpenter knows what types of nodes we want for unscheduled workloads.
You can refer to some of the [example provisioners](https://github.com/aws/karpenter/tree{{< githubRelRef >}}examples/provisioner) for specific needs.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  provider:
    subnetSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
    securityGroupSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
EOF

```

## Set nodeAffinity for critical workloads (optional)

You may also want to set a nodeAffinity for other critical cluster workloads.

Some examples are

* coredns
* metric-server

You can edit them with `kubectl edit deploy ...` and you should add node affinity for your static node group instances.
Modify the `ng-123456` value to match your `$NODEGROUP`.

```
      affinity:                      
        nodeAffinity: 
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: eks.amazonaws.com/nodegroup
                operator: In
                values:
                - ng-123456
```

## Remove CAS

Now that karpenter is running we can disable the cluster autoscaler.
To do that we will scale the number of replicas to zero.

```bash
kubectl scale delpoy/cluster-autoscaler -n kube-system --replicas=0
```

To get rid of the instances that were added from the node group we can scale our nodegroup down to a minimum size to support Karpenter and other critical services.
We suggest a minimum of 2 nodes for the node group.

> Note: If your workloads do not have [pod disruption budgets](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) set
> the following command **will cause workloads to be unavailable**

```bash
aws eks update-nodegroup-config --cluster-name ${CLUSTER_NAME} \
    --nodegroup-name ${NODEGROUP} \
    --scaling-config "minSize=2,maxSize=2,desiredSize=2"
```

If you have a lot of nodes or workloads you may want to slowly step down your node groups by a few instances at a time.
It is recommended to watch the transition carefully for workloads that may not have enough replicas running or disruption budgets configured.

## Verify Karpenter

As nodegroup nodes are drained you can verify that Karpenter is creating nodes for your workloads.

```bash
kubectl logs -f -n karpenter -c controller -l app.kubernetes.io/name=karpenter
```

You should also see new nodes created in your cluster as the old nodes are removed
```bash
kubectl get nodes
```