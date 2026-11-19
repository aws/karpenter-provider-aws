# Download the 4 controller policy templates from the doc's policies/
# directory (same versions as the CloudFormation template would create,
# minus the InterruptionPolicy and ZonalShiftPolicy which require SQS +
# EventBridge that this minimal path skips).
BASE_URL="https://raw.githubusercontent.com/aws/karpenter-provider-aws/v{{< param "latest_release_version" >}}/website/content/en/preview/getting-started/adding-karpenter-to-existing-eks/policies"

for logical in NodeLifecyclePolicy IAMIntegrationPolicy EKSIntegrationPolicy ResourceDiscoveryPolicy; do
  POLICY_NAME="KarpenterController${logical}-${CLUSTER_NAME}"
  echo "Creating ${POLICY_NAME}..."
  curl -fsSL "${BASE_URL}/${logical}.json" | envsubst > /tmp/${logical}.json
  aws iam create-policy \
    --policy-name "${POLICY_NAME}" \
    --policy-document "file:///tmp/${logical}.json"
done
