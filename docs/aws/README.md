
# AWS
This guide will provide a complete Karpenter installation for AWS. These steps are opinionated and may need to be adapted for your use case.
## Environment
```bash
CLOUD_PROVIDER=aws
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
CLUSTER_NAME=$USER-karpenter-demo
AWS_DEFAULT_REGION=us-west-2
```

### Create a Cluster
Skip this step if you already have a cluster.
```bash
eksctl create cluster \
--name ${CLUSTER_NAME} \
--version 1.18 \
--region ${AWS_DEFAULT_REGION} \
--node-type m5.large \
--nodes 1 \
--nodes-min 1 \
--nodes-max 10 \
--managed
```

### Create IAM Resources
This command will create IAM resources used by Karpenter. We recommend using [CloudFormation](https://aws.amazon.com/cloudformation/) and [IAM Roles for Service Accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) (IRSA) to manage these permissions. For production use, please review and restrict these permissions for your use case.
```bash
TEMPOUT=$(mktemp)
curl -fsSL https://raw.githubusercontent.com/awslabs/karpenter/v0.2.4/docs/aws/karpenter.cloudformation.yaml > $TEMPOUT \
| aws cloudformation deploy \
 --stack-name Karpenter-${CLUSTER_NAME} \
 --template-file ${TEMPOUT}\
 --capabilities CAPABILITY_NAMED_IAM \
 --parameter-overrides ClusterName=${CLUSTER_NAME} OpenIDConnectIdentityProvider=$(aws eks describe-cluster --name ${CLUSTER_NAME} | jq -r ".cluster.identity.oidc.issuer" | cut -c9-)
```

### Install Karpenter Controller and Dependencies
Karpenter relies on [cert-manager](https://github.com/jetstack/cert-manager) for Webhook TLS certificates.

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/awslabs/karpenter/v0.2.4/hack/quick-install.sh)"
```

### Setup IRSA, Karpenter Controller Role, and Karpenter Node Role
```bash
# Enables IRSA for your cluster. This command is idempotent, but only needs to be executed once per cluster.
eksctl utils associate-iam-oidc-provider \
--region ${AWS_DEFAULT_REGION} \
--cluster ${CLUSTER_NAME} \
--approve

# Setup KarpenterControllerRole
kubectl patch serviceaccount karpenter -n karpenter --patch "$(cat <<-EOM
metadata:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterControllerRole-${CLUSTER_NAME}
EOM
)"

# Setup KarpenterNodeRole
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

# Restart controller to load credentials
kubectl delete pods -n karpenter -l control-plane=karpenter
```

### (Optional) Enable Verbose Logging
```bash
kubectl patch deployment karpenter -n karpenter --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/args", "value": ["--verbose"]}]'
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
    caBundle: $(aws eks describe-cluster --name ${CLUSTER_NAME} | jq ".cluster.certificateAuthority.data")
    endpoint: $(aws eks describe-cluster --name ${CLUSTER_NAME} | jq ".cluster.endpoint")
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
kubectl logs -f -n karpenter $(kubectl get pods -n karpenter -l control-plane=karpenter -ojson | jq -r ".items[0].metadata.name")
```

### Cleanup
```bash
./hack/quick-install.sh --delete
aws cloudformation delete-stack --stack-name Karpenter-${CLUSTER_NAME}
aws ec2 describe-launch-templates | jq -r ".LaunchTemplates[].LaunchTemplateName" | grep Karpenter | xargs -I{} aws ec2 delete-launch-template --launch-template-name {}
```
