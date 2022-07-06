TEMPOUT=$(mktemp)
curl -fsSL https://raw.githubusercontent.com/awslabs/kubernetes-iteration-toolkit/main/operator/docs/kit.cloudformation.yaml  > $TEMPOUT \
&& aws cloudformation deploy  \
  --template-file ${TEMPOUT} \
  --capabilities CAPABILITY_NAMED_IAM \
  --stack-name kitControllerPolicy-${CLUSTER_NAME} \
  --parameter-overrides ClusterName=${CLUSTER_NAME}

eksctl create iamserviceaccount \
  --name kit-controller \
  --namespace kit \
  --cluster ${CLUSTER_NAME} \
  --attach-policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/KitControllerPolicy-${CLUSTER_NAME} \
  --approve \
  --override-existing-serviceaccounts \
  --region=${AWS_REGION}

helm repo add kit https://awslabs.github.io/kubernetes-iteration-toolkit/
helm upgrade --install kit-operator kit/kit-operator --namespace kit --create-namespace --set serviceAccount.create=false
