# AWS

This guide will provide a complete Karpenter installation for AWS.
These steps are opinionated and may need to be adapted for your use case.

## Environment
```bash
CLOUD_PROVIDER=aws
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
CLUSTER_NAME=$USER-karpenter-demo
AWS_DEFAULT_REGION=us-west-2
export AWS_DEFAULT_OUTPUT=json
```

### Create a Cluster

Create an EKS cluster
```bash
eksctl create cluster \
--name ${CLUSTER_NAME} \
--node-type m5.large \
--nodes 1 \
--nodes-min 1 \
--nodes-max 10 \
--managed \
--with-oidc
```

Tag the cluster subnets with the required tags for Karpenter auto discovery.

Note: If you have a cluster with version 1.18 or below you can skip this step.
More [detailed here](https://github.com/awslabs/karpenter/issues/404#issuecomment-845283904).

```bash
export SUBNET_IDS=$(aws cloudformation describe-stacks \
    --stack-name eksctl-${CLUSTER_NAME}-cluster \
    --query 'Stacks[].Outputs[?OutputKey==`SubnetsPrivate`].OutputValue' \
    --output text)

aws ec2 create-tags \
    --resources $(echo $SUBNET_IDS | tr ',' '\n') \
    --tags Key="kubernetes.io/cluster/${CLUSTER_NAME}",Value=
```

### Setup IRSA, Karpenter Controller Role, and Karpenter Node Role
We recommend using [CloudFormation](https://aws.amazon.com/cloudformation/) and [IAM Roles for Service Accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) (IRSA) to manage these permissions.
For production use, please review and restrict these permissions for your use case.

Note: For IRSA to work your [cluster needs an OIDC provider](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html)

```bash
export OIDC_PROVIDER=$(aws eks describe-cluster \
    --name ${CLUSTER_NAME} \
    --query 'cluster.identity.oidc.issuer' \
    --output text \
    | sed 's,https://,,')

# Creates IAM resources used by Karpenter
aws cloudformation deploy \
  --stack-name Karpenter-${CLUSTER_NAME} \
  --template-file  $(git rev-parse --show-toplevel)/docs/aws/karpenter.cloudformation.yaml \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides ClusterName=${CLUSTER_NAME} OpenIDConnectIdentityProvider=${OIDC_PROVIDER}

# Adds the karpenter node role to your aws-auth configmap, allowing nodes with this role to connect to the cluster.
kubectl patch configmap aws-auth -n kube-system --patch "$(cat <<-EOM
data:
  mapRoles: |
    - rolearn: arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME}
      username: system:node:{{EC2PrivateDNSName}}
      groups:
      - system:bootstrappers
      - system:nodes
$(kubectl get configmap -n kube-system aws-auth -ojsonpath='{.data.mapRoles}' | sed 's/^/    /')
EOM
)"
```

### Install Karpenter
```bash
helm repo add karpenter https://awslabs.github.io/karpenter/charts
helm repo update
# For additional values, see https://github.com/awslabs/karpenter/blob/main/charts/karpenter/values.yaml
helm upgrade --install karpenter charts/karpenter --create-namespace --namespace karpenter \
  --set serviceAccount.annotations.'eks\.amazonaws\.com/role-arn'=arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterControllerRole-${CLUSTER_NAME}
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
apiVersion: provisioning.karpenter.sh/v1alpha1
kind: Provisioner
metadata:
  name: default
spec:
  cluster:
    name: ${CLUSTER_NAME}
    caBundle: $(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.certificateAuthority.data")
    endpoint: $(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.endpoint")
EOF
kubectl get provisioner default -oyaml
```

### Create some pods
Create some dummy pods and observe logs.
> Note: this will cause EC2 Instances to launch, which will be billed to your AWS Account.
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

### Cleanup
```bash
helm delete karpenter -n karpenter
aws cloudformation delete-stack --stack-name Karpenter-${CLUSTER_NAME}
aws ec2 describe-launch-templates \
    | jq -r ".LaunchTemplates[].LaunchTemplateName" \
    | grep -i karpenter \
    | xargs -I{} aws ec2 delete-launch-template --launch-template-name {}
unset AWS_DEFAULT_OUTPUT
```

If you created a cluster during this process you also will need to delete the cluster.
```bash
eksctl delete cluster --name ${CLUSTER_NAME}
```

