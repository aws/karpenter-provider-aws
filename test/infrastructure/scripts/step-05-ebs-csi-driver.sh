TEMPOUT=$(mktemp)
echo "Installing AWS EBS CSI Driver"
if ! aws iam get-policy --policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/Karpenter-AmazonEBSCSIDriverServiceRolePolicy-${CLUSTER_NAME}; then
  echo "Creating AWS EBS CSI Driver IAM Policy"
  curl -o ${TEMPOUT} https://raw.githubusercontent.com/kubernetes-sigs/aws-ebs-csi-driver/v1.5.1/docs/example-iam-policy.json
  aws iam create-policy \
    --policy-name Karpenter-AmazonEBSCSIDriverServiceRolePolicy-${CLUSTER_NAME} \
    --policy-document file://${TEMPOUT}
fi

eksctl create iamserviceaccount \
    --name=ebs-csi-controller-sa \
    --namespace=kube-system \
    --cluster=${CLUSTER_NAME} \
    --attach-policy-arn=arn:aws:iam::${AWS_ACCOUNT_ID}:policy/Karpenter-AmazonEBSCSIDriverServiceRolePolicy-${CLUSTER_NAME} \
    --approve \
    --override-existing-serviceaccounts

helm repo add aws-ebs-csi-driver https://kubernetes-sigs.github.io/aws-ebs-csi-driver
helm repo update
helm upgrade --install aws-ebs-csi-driver \
    --namespace kube-system \
    --set controller.replicaCount=1 \
    --set controller.serviceAccount.create=false \
    aws-ebs-csi-driver/aws-ebs-csi-driver
