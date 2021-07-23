
---
title: "Getting Started with Karpenter on AWS"
linkTitle: "Getting Started Guide"
weight: 10
menu:
  main:
    weight: 10
---

Karpenter automatically provisions new nodes in response to unschedulable
pods. Karpenter does this by observing events within the Kubernetes cluster,
and then sending commands to the underlying cloud provider. 

In this example, the cluster is running on Amazon Web Services (AWS) Elastic
Kubernetes Service (EKS). Karpenter is designed to be cloud provider agnostic,
but currently only supports AWS. Contributions are welcomed. 

This guide should take less than 1 hour to complete, and cost less than $0.25.
Follow the clean-up instructions to reduce any charges.

## Install

Karpenter is installed in clusters with a helm chart.

Karpenter additionally requires IAM Roles for Service Accounts (IRSA). IRSA
permits Karpenter (within the cluster) to make privileged requests to AWS (as
the cloud provider). 

### Required Utilities

Install these tools before proceeding:

1. [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2-linux.html)
2. `kubectl` - [the kubernetes CLI](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/)
3. `eksctl` - [the CLI for AWS EKS](https://docs.aws.amazon.com/eks/latest/userguide/eksctl.html)

Login to the AWS CLI with a user that has sufficient privileges to create a
cluster. 

### Environment Variables

After setting up the tools, set the following environment variables to store
commonly used values. 

```bash
export CLUSTER_NAME=$USER-karpenter-demo
export AWS_DEFAULT_REGION=us-west-2
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
KARPENTER_VERSION=$(curl -fsSL \
  https://api.github.com/repos/awslabs/karpenter/releases/latest \
  | jq -r '.tag_name')
```

### Create a Cluster

Create a cluster with `eksctl`. The [example configuration](eks-config.yaml) file specifies a basic cluster (name, region), and an IAM role for Karpenter to use. 

```bash
curl -fsSL  https://raw.githubusercontent.com/awslabs/karpenter/"${KARPENTER_VERSION}"/pkg/cloudprovider/aws/docs/eks-config.yaml \
  | envsubst \
  | eksctl create cluster -f -
```

This guide uses a regular (un-managed) node group to host Karpenter.

Karpenter itself can run anywhere, including on self-managed node groups, [managed node groups](https://docs.aws.amazon.com/eks/latest/userguide/managed-node-groups.html), or [AWS Fargate](https://aws.amazon.com/fargate/).

Karpenter will provision new traditional instances on EC2. 

Additionally, the configuration file sets up an [OIDC
provider](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#openid-connect-tokens),
necessary for IRSA (see below). Kubernetes supports OIDC as a standardized way
of communicating with identity providers. 

### Setup Authentication from Kubernetes to AWS (IRSA)

IAM Roles for Service Accounts (IRSA) maps Kubernetes resources to roles
(permission sets) on AWS. 

First, define a role using the template below. It provides full access to EC2,
and limited access to other services such as EKS and Elastic Container Registry
(ECR).

```bash
# Creates IAM resources used by Karpenter
TEMPOUT=$(mktemp)
curl -fsSL https://raw.githubusercontent.com/awslabs/karpenter/"${KARPENTER_VERSION}"/docs/aws/karpenter.cloudformation.yaml > $TEMPOUT \
&& aws cloudformation deploy \
  --stack-name Karpenter-${CLUSTER_NAME} \
  --template-file ${TEMPOUT} \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides ClusterName=${CLUSTER_NAME}
```

Second, create the mapping between Kubernetes resources and the new IAM role. 

```bash
# Add the Karpenter node role to your aws-auth configmap, allowing nodes with this role to connect to the cluster.
eksctl create iamidentitymapping \
  --username system:node:{{EC2PrivateDNSName}} \
  --cluster  ${CLUSTER_NAME} \
  --arn arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME} \
  --group system:bootstrappers \
  --group system:nodes
```

Now, Karpenter can send requests for new EC2 instances to AWS and those instances can connect to your cluster. 

### Install Karpenter Helm Chart

Use helm to deploy Karpenter to the cluster. 

We created a Kubernetes service account when we created the cluster using
eksctl. Thus, we don't need the helm chart to do that.

```bash
helm repo add karpenter https://awslabs.github.io/karpenter/charts
helm repo update
helm upgrade --install karpenter karpenter/karpenter \
  --namespace karpenter --set serviceAccount.create=false
```

### Provisioner

A single Karpenter provisioner is capable of handling many different pod
shapes. In other words, Karpenter eliminates the need to manage many different
node groups. Karpenter makes scheduling and provisioning decisions based on pod
attributes such as labels and affinity. 

Create a simple default provisioner using the command below. This provisioner
provides instances with the default certificate bundle, and the control plane
endpoint url. 

Importantly, the `ttlSecondsAfterEmpty` value configures Karpenter to
deprovision empty nodes. This behavior can be disabled by leaving the value
undefined. 

Review the [provsioner CRD](/docs/provisioner-crd) for more information. For example, 
`ttlSecondsUntilExpired` configures Karpenter to deprovision
nodes when a maximum age is reached. 


```bash
cat <<EOF | kubectl apply -f -
apiVersion: karpenter.sh/v1alpha3
kind: Provisioner
metadata:
  name: default
spec:
  cluster:
    name: ${CLUSTER_NAME}
    endpoint: $(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.endpoint" --output json)
  ttlSecondsAfterEmpty: 30
EOF
kubectl get provisioner default -o yaml
```

## First Use

Karpenter is now active and ready to begin provisioning nodes. Create a
workload (e.g., deployment) to see Karpenter provision some nodes. 

### Create Workload

This deployment uses the [pause image](https://www.ianlewis.org/en/almighty-pause-container) and starts with zero replicas. 

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
```
### Automatic Node Provisioning 

Scale the previous deployment to 5 replicas, and begin watching the Karpenter
pod for log events. 

```bash
kubectl scale deployment inflate --replicas 5
kubectl logs -f -n karpenter $(kubectl get pods -n karpenter -l karpenter=controller -o name)
```

### Automatic Node Deprovisioning (timeout)

Now, delete the deployment. After 30 seconds (`ttlSecondsAfterEmpty`),
Karpenter should deprovision the now empty nodes. 

```bash
kubectl delete deployment inflate
kubectl logs -f -n karpenter $(kubectl get pods -n karpenter -l karpenter=controller -o name)
```

### Manual Node Deprovisioning

If you delete a node with kubectl, Karpenter terminates the corresponding instance. More specifically, Karpenter adds a node finalizer to properly cordon and drain nodes before they are terminated.

```bash
kubectl delete node $NODE_NAME
```

## Cleanup

It's important to both delete the cluster instances and the cluster control
plane. AWS charges for both. 

Delete cluster workloads and instances: 

```bash
helm delete karpenter -n karpenter
aws cloudformation delete-stack --stack-name Karpenter-${CLUSTER_NAME}
aws ec2 describe-launch-templates \
    | jq -r ".LaunchTemplates[].LaunchTemplateName" \
    | grep -i Karpenter-${CLUSTER_NAME} \
    | xargs -I{} aws ec2 delete-launch-template --launch-template-name {}
```

Delete the control plane: 

```bash
eksctl delete cluster --name ${CLUSTER_NAME}
```
