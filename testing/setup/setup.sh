# Environment Variables Config
export CLUSTER_NAME=kit-management-cluster
export GUEST_CLUSTER_NAME=example
export AWS_PROFILE=karpenter-ci
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
export AWS_REGION=us-west-2
export KARPENTER_VERSION=v0.10.1

# Create Cluster
cat <<EOF > cluster.yaml
---
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: ${CLUSTER_NAME}
  region: ${AWS_REGION}
  version: "1.21"
  tags:
    karpenter.sh/discovery: ${CLUSTER_NAME}
managedNodeGroups:
  - instanceType: m5.large
    amiFamily: AmazonLinux2
    name: ${CLUSTER_NAME}-ng
    desiredCapacity: 1
    minSize: 1
    maxSize: 10
iam:
  withOIDC: true
EOF
eksctl create cluster -f cluster.yaml

# Tekton
kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.33.2/release.yaml
kubectl apply -f https://storage.googleapis.com/tekton-releases/triggers/previous/v0.19.0/release.yaml
kubectl apply -f https://github.com/tektoncd/dashboard/releases/download/v0.24.1/tekton-dashboard-release.yaml

# AWS Load balancer
curl -o iam_policy.json https://raw.githubusercontent.com/kubernetes-sigs/aws-load-balancer-controller/v2.3.1/docs/install/iam_policy.json
aws iam create-policy \
    --policy-name AWSLoadBalancerControllerIAMPolicy \
    --policy-document file://iam_policy.json

eksctl create iamserviceaccount \
  --cluster=${CLUSTER_NAME} \
  --namespace=kube-system \
  --name=aws-load-balancer-controller \
  --attach-policy-arn=arn:aws:iam::${AWS_ACCOUNT_ID}:policy/AWSLoadBalancerControllerIAMPolicy \
  --override-existing-serviceaccounts \
  --approve

helm repo add eks https://aws.github.io/eks-charts
helm repo update
helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=${CLUSTER_NAME} \
  --set serviceAccount.create=false \
  --set replicaCount=1 \
  --set serviceAccount.name=aws-load-balancer-controller

# AWS EBS CSI Driver
curl -o example-iam-policy.json https://raw.githubusercontent.com/kubernetes-sigs/aws-ebs-csi-driver/v1.5.1/docs/example-iam-policy.json
aws iam create-policy \
    --policy-name AmazonEBSCSIDriverServiceRolePolicy \
    --policy-document file://example-iam-policy.json

eksctl create iamserviceaccount \
    --name=ebs-csi-controller-sa \
    --namespace=kube-system \
    --cluster=${CLUSTER_NAME} \
    --attach-policy-arn=arn:aws:iam::${AWS_ACCOUNT_ID}:policy/AmazonEBSCSIDriverServiceRolePolicy \
    --approve \
    --override-existing-serviceaccounts \
    --role-name AmazonEKS_EBS_CSI_DriverRole

helm repo add aws-ebs-csi-driver https://kubernetes-sigs.github.io/aws-ebs-csi-driver
helm repo update
helm upgrade --install aws-ebs-csi-driver \
    --namespace kube-system \
    --set controller.replicaCount=1 \
    --set controller.serviceAccount.create=false \
    aws-ebs-csi-driver/aws-ebs-csi-driver

# Install Karpenter
TEMPOUT=$(mktemp)

aws cloudformation deploy \
  --stack-name "Karpenter-${CLUSTER_NAME}" \
  --template-file host-cluster.cloudformation.yaml \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides "ClusterName=${CLUSTER_NAME}"

ROLE="    - rolearn: arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME}\n      username: system:node:{{EC2PrivateDNSName}}\n      groups:\n      - system:nodes\n      - system:bootstrappers"
kubectl get -n kube-system configmap/aws-auth -o yaml | awk "/mapRoles: \|/{print;print \"$ROLE\";next}1" > /tmp/aws-auth-patch.yml
kubectl patch configmap/aws-auth -n kube-system --patch "$(cat /tmp/aws-auth-patch.yml)"

eksctl create iamserviceaccount \
  --cluster "${CLUSTER_NAME}" --name karpenter --namespace karpenter \
  --role-name "${CLUSTER_NAME}-karpenter" \
  --attach-policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/KarpenterControllerPolicy-${CLUSTER_NAME}" \
  --role-only \
  --approve

export KARPENTER_IAM_ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"

aws iam create-service-linked-role --aws-service-name spot.amazonaws.com || true

helm repo add karpenter https://charts.karpenter.sh
helm repo update
helm upgrade --install --namespace karpenter --create-namespace \
  karpenter karpenter/karpenter \
  --version ${KARPENTER_VERSION} \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter" \
  --set clusterName=${CLUSTER_NAME} \
  --set clusterEndpoint=$(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.endpoint" --output json)  \
  --wait

# Provisioner for KIT Guest Clusters
cat <<EOF | kubectl apply -f -
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  requirements:
  - key: kit.k8s.sh/app
    operator: Exists
  - key: kit.k8s.sh/control-plane-name
    operator: Exists
  ttlSecondsAfterEmpty: 30
  provider:
    instanceProfile: KarpenterNodeInstanceProfile-${CLUSTER_NAME}
    tags:
      kit.aws/substrate: ${CLUSTER_NAME}
    subnetSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
    securityGroupSelector:
      kubernetes.io/cluster/${CLUSTER_NAME}: owned
EOF

# Provisioner for Tekton Pods
cat << EOF | kubectl apply -f -
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: tekton-provisioner
spec:
  ttlSecondsAfterEmpty: 30
  requirements:
  - key: "kubernetes.io/arch"
    operator: In
    values: ["amd64"]
  provider:
    instanceProfile: KarpenterControllerPolicy-${CLUSTER_NAME}
    subnetSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
    securityGroupSelector:
      kubernetes.io/cluster/${CLUSTER_NAME}: owned
EOF
