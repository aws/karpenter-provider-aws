# AWS

This guide will provide a complete Karpenter installation for AWS.
These steps are opinionated and may need to be adapted for your use case.

> This guide should take less than 1 hour to complete and cost less than $.25

## Environment
```bash
export CLUSTER_NAME=$USER-karpenter-demo
export AWS_DEFAULT_REGION=us-west-2
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
KARPENTER_VERSION=$(curl -fsSL \
  https://api.github.com/repos/awslabs/karpenter/releases/latest \
  | jq -r '.tag_name')
```

### Create a Cluster

Karpenter can run anywhere, including on self-managed node groups, [managed node groups](https://docs.aws.amazon.com/eks/latest/userguide/managed-node-groups.html), or [AWS Fargate](https://aws.amazon.com/fargate/).
This demo will run Karpenter on Fargate, which means all EC2 instances added to this cluster will be controlled by Karpenter.

```bash
curl -fsSL https://raw.githubusercontent.com/awslabs/karpenter/"${KARPENTER_VERSION}"/docs/aws/eks-config.yaml \
  | envsubst \
  | eksctl create cluster -f -
```

Tag the cluster subnets with the required tags for Karpenter auto discovery.

> If you are using a cluster with version 1.18 or below you can skip this step.
More [detailed here](https://github.com/awslabs/karpenter/issues/404#issuecomment-845283904).

```bash
SUBNET_IDS=$(aws cloudformation describe-stacks \
    --stack-name eksctl-${CLUSTER_NAME}-cluster \
    --query 'Stacks[].Outputs[?OutputKey==`SubnetsPrivate`].OutputValue' \
    --output text)

aws ec2 create-tags \
    --resources $(echo ${SUBNET_IDS//,/ }) \
    --tags Key="kubernetes.io/cluster/${CLUSTER_NAME}",Value=
```

### Setup IRSA, Karpenter Controller Role, and Karpenter Node Role
We recommend using [CloudFormation](https://aws.amazon.com/cloudformation/) and [IAM Roles for Service Accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) (IRSA) to manage these permissions.
For production use, please review and restrict these permissions for your use case.

> For IRSA to work your cluster needs an [OIDC provider](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html)

```bash
# Creates IAM resources used by Karpenter
TEMPOUT=$(mktemp)
curl -fsSL https://raw.githubusercontent.com/awslabs/karpenter/"${KARPENTER_VERSION}"/docs/aws/karpenter.cloudformation.yaml > $TEMPOUT \
&& aws cloudformation deploy \
  --stack-name Karpenter-${CLUSTER_NAME} \
  --template-file ${TEMPOUT} \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides ClusterName=${CLUSTER_NAME}

# Add the karpenter node role to your aws-auth configmap, allowing nodes with this role to connect to the cluster.
eksctl create iamidentitymapping \
  --username system:node:{{EC2PrivateDNSName}} \
  --cluster  ${CLUSTER_NAME} \
  --arn arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME} \
  --group system:bootstrappers \
  --group system:nodes
```

### Install Karpenter

Use [`helm`](https://helm.sh/) to deploy Karpenter to the cluster.
For additional values, see [the helm chart values](https://github.com/awslabs/karpenter/blob/main/charts/karpenter/values.yaml)

> We created a Kubernetes service account with our cluster so we don't need the helm chart to do that.

```bash
helm repo add karpenter https://awslabs.github.io/karpenter/charts
helm repo update
helm upgrade --install karpenter karpenter/karpenter \
  --namespace karpenter --set serviceAccount.create=false
```

### (Optional) Enable Verbose Logging
```bash
kubectl patch deployment karpenter-controller \
    -n karpenter --type='json' \
    -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/args", "value": ["--verbose"]}]'
```

### Create a Provisioner
Create a default Provisioner that launches nodes configured with cluster name, endpoint, and caBundle.
```bash
cat <<EOF | kubectl apply -f -
apiVersion: provisioning.karpenter.sh/v1alpha2
kind: Provisioner
metadata:
  name: default
spec:
  ttlSeconds: 30
  cluster:
    name: ${CLUSTER_NAME}
    caBundle: $(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.certificateAuthority.data" --output json)
    endpoint: $(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.endpoint" --output json)
  ttlSecondsAfterEmpty: 30
EOF
kubectl get provisioner default -o yaml
```

### Create some pods
Create some dummy pods and observe logs.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: inflate
spec:
  replicas: 0
  selector:
    matchLabels:
      app: inflate
  template:
    metadata:
      labels:
        app: inflate
    spec:
      containers:
        - name: inflate
          image: public.ecr.aws/eks-distro/kubernetes/pause:3.2
          resources:
            requests:
              cpu: 1
EOF
kubectl scale deployment inflate --replicas 5
kubectl logs -f -n karpenter $(kubectl get pods -n karpenter -l karpenter=controller -o name)
```

You can see what EC2 instance type was added to your cluster from Karpenter with
```bash
kubectl get no -L "node.kubernetes.io/instance-type"
```

If you scale down the deployment replicas the instance will be terminated after 30 seconds (ttlSeconds).
```bash
kubectl scale deployment inflate --replicas 0
```

Or you can manually delete the node with

> Karpenter automatically adds a node finalizer to properly cordon and drain nodes before they are terminated.
```bash
kubectl delete node $NODE_NAME
```

### Cleanup
> To avoid additional costs make sure you delete all ec2 instances before deleting the other cluster resources.
```bash
helm delete karpenter -n karpenter
aws cloudformation delete-stack --stack-name Karpenter-${CLUSTER_NAME}
aws ec2 describe-launch-templates \
    | jq -r ".LaunchTemplates[].LaunchTemplateName" \
    | grep -i Karpenter-${CLUSTER_NAME} \
    | xargs -I{} aws ec2 delete-launch-template --launch-template-name {}
```

If you created a cluster during this process you also will need to delete the cluster.
```bash
eksctl delete cluster --name ${CLUSTER_NAME}
```
