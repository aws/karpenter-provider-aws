if [ -z "$(kubectl get ns tekton-tests)" ]; then
  kubectl create ns tekton-tests
fi

kubectl apply -f ${SCRIPTPATH}/rbac.yaml

eksctl create iamserviceaccount \
  --cluster "${CLUSTER_NAME}" --name tekton --namespace tekton-tests \
  --role-name "${CLUSTER_NAME}-tekton" \
  --attach-policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/TektonPodPolicy-${CLUSTER_NAME}" \
  --role-only \
  --override-existing-serviceaccounts \
  --approve

ROLE="    - rolearn: arn:aws:sts::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-tekton\n      username: system:node:{{EC2PrivateDNSName}}\n      groups:\n      - tekton"
kubectl get -n kube-system configmap/aws-auth -o yaml | awk "/mapRoles: \|/{print;print \"${ROLE}\";next}1" >/tmp/aws-auth-patch.yml
kubectl patch configmap/aws-auth -n kube-system --patch "$(cat /tmp/aws-auth-patch.yml)"

kubectl annotate --overwrite serviceaccount -n tekton-tests tekton "eks.amazonaws.com/role-arn=arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-tekton"

echo "Installed IRSA for Tekton pods in tekton-tests namespace."
