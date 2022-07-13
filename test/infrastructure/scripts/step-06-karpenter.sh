echo "Installing Karpenter"

aws cloudformation deploy \
  --stack-name "KarpenterTestInfrastructure-${CLUSTER_NAME}" \
  --template-file ${SCRIPTPATH}/management-cluster.cloudformation.yaml \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides "ClusterName=${CLUSTER_NAME}"

ROLE="    - rolearn: arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME}\n      username: system:node:{{EC2PrivateDNSName}}\n      groups:\n      - system:nodes\n      - system:bootstrappers"
kubectl get -n kube-system configmap/aws-auth -o yaml | awk "/mapRoles: \|/{print;print \"${ROLE}\";next}1" >/tmp/aws-auth-patch.yml
kubectl patch configmap/aws-auth -n kube-system --patch "$(cat /tmp/aws-auth-patch.yml)"

eksctl create iamserviceaccount \
  --cluster "${CLUSTER_NAME}" --name karpenter --namespace karpenter \
  --role-name "${CLUSTER_NAME}-karpenter" \
  --attach-policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/KarpenterControllerPolicy-${CLUSTER_NAME}" \
  --role-only \
  --approve

export KARPENTER_IAM_ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"

aws iam create-service-linked-role --aws-service-name spot.amazonaws.com || true

if [ -z "$(helm repo list | grep karpenter)" ]; then
  helm repo add karpenter https://charts.karpenter.sh
fi
helm repo update
helm upgrade --install --namespace karpenter --create-namespace karpenter --version v0.13.2 \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter" \
  --set clusterName=${CLUSTER_NAME} \
  --set clusterEndpoint=$(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.endpoint" --output json) \
  --set aws.defaultInstanceProfile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
  --set tolerations[0].key="CriticalAddonsOnly" \
  --set tolerations[0].operator="Exists" \
  karpenter/karpenter \
  --wait
