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
aws cloudformation wait stack-delete-complete \
  --stack-name "${EKSCTL_IRSA_STACK}" 2>/dev/null || true

# 5. Detach controller policies from any role and delete them.
#    (No CloudFormation stack to drop; we created these directly.)
for role in $(aws iam list-policies --scope Local \
  --query "Policies[?contains(PolicyName, 'KarpenterController') && ends_with(PolicyName, '${CLUSTER_NAME}')].Arn" \
  --output text | tr '\t' '\n'); do
  for entity in $(aws iam list-entities-for-policy --policy-arn "$role" \
    --query 'PolicyRoles[].RoleName' --output text | tr '\t' '\n'); do
    [ -n "$entity" ] && aws iam detach-role-policy --role-name "$entity" --policy-arn "$role"
  done
  aws iam delete-policy --policy-arn "$role"
done

# 6. Detach managed policies from KarpenterNodeRole and delete the role.
NODE_ROLE="KarpenterNodeRole-${CLUSTER_NAME}"
aws iam list-attached-role-policies --role-name "${NODE_ROLE}" \
  --query 'AttachedPolicies[].PolicyArn' --output text 2>/dev/null \
  | tr '\t' '\n' \
  | xargs -r -I{} aws iam detach-role-policy --role-name "${NODE_ROLE}" --policy-arn {}
aws iam delete-role --role-name "${NODE_ROLE}" 2>/dev/null || true

# 7. Untag subnets and security groups (harmless once Karpenter is gone,
#    but worth removing if you reuse the cluster name later)
aws ec2 delete-tags \
  --resources "${SUBNETS[@]}" "${CLUSTER_SG}" \
  --tags "Key=karpenter.sh/discovery,Value=${CLUSTER_NAME}"

# 8. Delete any leftover launch templates Karpenter created
aws ec2 describe-launch-templates \
  --filters "Name=tag:karpenter.k8s.aws/cluster,Values=${CLUSTER_NAME}" \
  --query 'LaunchTemplates[].LaunchTemplateName' --output text \
  | tr '\t' '\n' \
  | xargs -r -n1 aws ec2 delete-launch-template --launch-template-name
