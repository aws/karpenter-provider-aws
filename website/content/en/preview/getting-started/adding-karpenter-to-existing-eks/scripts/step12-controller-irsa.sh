# Discover the controller policies. AWS CLI returns multiple ARNs
# tab-separated and wraps lines as the count grows, so pipe through `tr`
# to put one ARN per line, then read into an array with a while loop
# (portable across bash 3.2+). Earlier patterns using `read -ra` only
# consumed the first line and silently dropped policies past index 4.
CONTROLLER_POLICIES=()
while IFS= read -r arn; do
  CONTROLLER_POLICIES+=("$arn")
done < <(aws iam list-policies \
  --scope Local \
  --query "Policies[?starts_with(PolicyName, 'KarpenterController') && ends_with(PolicyName, '${CLUSTER_NAME}')].Arn" \
  --output text | tr '\t' '\n')

# Sanity check: the CloudFormation template creates 6 controller policies.
# If you see fewer here, Step 3's stack didn't create them all.
echo "Discovered ${#CONTROLLER_POLICIES[@]} controller policies (expect 6):"
printf '  %s\n' "${CONTROLLER_POLICIES[@]}"

# Build the --attach-policy-arn flag pairs explicitly.
ATTACH_ARGS=()
for arn in "${CONTROLLER_POLICIES[@]}"; do
  ATTACH_ARGS+=(--attach-policy-arn "$arn")
done

eksctl create iamserviceaccount \
  --cluster "${CLUSTER_NAME}" \
  --namespace "${KARPENTER_NAMESPACE}" \
  --name karpenter \
  --role-name "KarpenterController-${CLUSTER_NAME}" \
  "${ATTACH_ARGS[@]}" \
  --role-only \
  --approve

export KARPENTER_IAM_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/KarpenterController-${CLUSTER_NAME}"
