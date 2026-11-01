# Identify any role still using the controller policies
aws iam list-policies --scope Local \
  --query "Policies[?starts_with(PolicyName, 'KarpenterController') && ends_with(PolicyName, '${CLUSTER_NAME}')].Arn" \
  --output text \
  | tr '\t' '\n' \
  | while read -r policy_arn; do
      aws iam list-entities-for-policy --policy-arn "${policy_arn}" \
        --query 'PolicyRoles[].RoleName' --output text
    done

# For each leftover role, detach all attached policies and delete
LEFTOVER_ROLE="<role-name-from-above>"
aws iam list-attached-role-policies --role-name "${LEFTOVER_ROLE}" \
  --query 'AttachedPolicies[].PolicyArn' --output text \
  | tr '\t' '\n' \
  | xargs -r -I{} aws iam detach-role-policy --role-name "${LEFTOVER_ROLE}" --policy-arn {}
aws iam delete-role --role-name "${LEFTOVER_ROLE}"

# Then retry the CFN stack delete
aws cloudformation delete-stack --stack-name "Karpenter-${CLUSTER_NAME}"
aws cloudformation wait stack-delete-complete --stack-name "Karpenter-${CLUSTER_NAME}"
