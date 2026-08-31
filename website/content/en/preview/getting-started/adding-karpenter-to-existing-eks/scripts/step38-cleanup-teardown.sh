# 1. Workload + Karpenter resources
kubectl delete deployment inflate --ignore-not-found
kubectl delete nodepool default --ignore-not-found
kubectl delete ec2nodeclass default --ignore-not-found

# 2. Karpenter controller (helm uninstall does NOT remove the CRDs)
helm uninstall karpenter --namespace "${KARPENTER_NAMESPACE}"
kubectl delete crd \
  ec2nodeclasses.karpenter.k8s.aws \
  nodeclaims.karpenter.sh \
  nodeoverlays.karpenter.sh \
  nodepools.karpenter.sh \
  --ignore-not-found

# 3. EKS access entry (silently no-op if missing)
aws eks delete-access-entry \
  --cluster-name "${CLUSTER_NAME}" \
  --principal-arn "arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME}" \
  2>/dev/null || true

# 4. eksctl-managed IRSA stack. eksctl sets TerminationProtection by
#    default, so disable it before deleting.
EKSCTL_IRSA_STACK="eksctl-${CLUSTER_NAME}-addon-iamserviceaccount-kube-system-karpenter"
aws cloudformation update-termination-protection \
  --no-enable-termination-protection \
  --stack-name "${EKSCTL_IRSA_STACK}" 2>/dev/null || true
aws cloudformation delete-stack --stack-name "${EKSCTL_IRSA_STACK}" 2>/dev/null || true

# 5. Karpenter IAM CloudFormation stack from Step 3
aws cloudformation delete-stack --stack-name "Karpenter-${CLUSTER_NAME}"

# Wait for both stacks to fully delete before continuing
aws cloudformation wait stack-delete-complete \
  --stack-name "${EKSCTL_IRSA_STACK}" 2>/dev/null || true
aws cloudformation wait stack-delete-complete \
  --stack-name "Karpenter-${CLUSTER_NAME}"

# 6. Untag subnets and security groups (harmless once Karpenter is gone,
#    but worth removing if you reuse the cluster name later)
aws ec2 delete-tags \
  --resources "${SUBNETS[@]}" "${CLUSTER_SG}" \
  --tags "Key=karpenter.sh/discovery,Value=${CLUSTER_NAME}"

# 7. Delete any leftover launch templates Karpenter created
aws ec2 describe-launch-templates \
  --filters "Name=tag:karpenter.k8s.aws/cluster,Values=${CLUSTER_NAME}" \
  --query 'LaunchTemplates[].LaunchTemplateName' --output text \
  | tr '\t' '\n' \
  | xargs -r -n1 aws ec2 delete-launch-template --launch-template-name
