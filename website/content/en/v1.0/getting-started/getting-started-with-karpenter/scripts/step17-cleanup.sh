kubectl delete nodepools --all
kubectl wait --for=delete nodeclaims --all --timeout=10m
kubectl delete ec2nodeclasses --all
helm uninstall karpenter --namespace "${KARPENTER_NAMESPACE}"
aws ec2 describe-launch-templates --filters "Name=tag:karpenter.k8s.aws/cluster,Values=${CLUSTER_NAME}" |
    jq -r ".LaunchTemplates[].LaunchTemplateName" |
    xargs -I{} aws ec2 delete-launch-template --launch-template-name {}
eksctl delete cluster --name "${CLUSTER_NAME}"
aws cloudformation delete-stack --stack-name "Karpenter-${CLUSTER_NAME}"
aws cloudformation wait stack-delete-complete --stack-name "Karpenter-${CLUSTER_NAME}"
