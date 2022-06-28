TEMPOUT=$(mktemp)
echo "Installing AWS Load Balancer"
if ! aws iam get-policy --policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/Karpenter-AWSLoadBalancerControllerIAMPolicy-${CLUSTER_NAME}; then
  echo "Creating AWS Load Balancer IAM Policy"
  curl -o ${TEMPOUT} https://raw.githubusercontent.com/kubernetes-sigs/aws-load-balancer-controller/v2.3.1/docs/install/iam_policy.json
  aws iam create-policy \
    --policy-name Karpenter-AWSLoadBalancerControllerIAMPolicy-${CLUSTER_NAME} \
    --policy-document file://${TEMPOUT}
fi

eksctl create iamserviceaccount \
  --cluster=${CLUSTER_NAME} \
  --namespace=kube-system \
  --name=aws-load-balancer-controller \
  --attach-policy-arn=arn:aws:iam::${AWS_ACCOUNT_ID}:policy/Karpenter-AWSLoadBalancerControllerIAMPolicy-${CLUSTER_NAME} \
  --override-existing-serviceaccounts \
  --approve

helm repo add eks https://aws.github.io/eks-charts
helm repo update
helm upgrade --install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=${CLUSTER_NAME} \
  --set serviceAccount.create=false \
  --set tolerations[0].key="CriticalAddonsOnly" \
  --set tolerations[0].operator="Exists" \
  --set replicaCount=1 \
  --set serviceAccount.name=aws-load-balancer-controller \
  eks/aws-load-balancer-controller
