
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
--nodegroup-name karpenter-aws-demo \
--node-type m5.large \
--nodes 1 \
--nodes-min 1 \
--nodes-max 10 \
--managed
```

### Create IAM Resources
This command will create IAM resources used by Karpenter. We recommend using Cloud Formation and IAM Roles for Service Accounts to manage these permissions. For production use, please review and restrict these permissions for your use case.
```bash
aws cloudformation deploy \
  --stack-name Karpenter \
  --template-file ./docs/aws/karpenter.cloudformation.yaml \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides OpenIDConnectIdentityProvider=$(aws eks describe-cluster --name ${CLUSTER_NAME} | jq -r ".cluster.identity.oidc.issuer" | cut -c9-)
```

### Install Karpenter Controller and Dependencies
Karpenter relies on cert-manager for certificates and prometheus for metrics.

```bash
./hack/quick-install.sh
```

### Enable IRSA and attach IAM Role to Service Account
Enables IRSA for your cluster. This command is idempotent, but only needs to be executed once per cluster.
```bash
eksctl utils associate-iam-oidc-provider \
--region ${AWS_DEFAULT_REGION} \
--cluster ${CLUSTER_NAME} \
--approve

kubectl patch serviceaccount karpenter -n karpenter --patch "$(cat <<-EOM
metadata:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterControllerRole
EOM
)"

kubectl delete pods -n karpenter -l control-plane=karpenter # Restart controller to load credentials
```

### Create a Provisoner
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
          image: k8s.gcr.io/pause
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
aws cloudformation delete-stack --stack-name Karpenter
aws ec2 describe-launch-templates | jq -r ".LaunchTemplates[].LaunchTemplateName" | grep Karpenter | xargs -I{} aws ec2 delete-launch-template --launch-template-name {}
```
