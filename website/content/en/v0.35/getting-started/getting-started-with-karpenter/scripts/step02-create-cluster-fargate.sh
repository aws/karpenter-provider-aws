eksctl create cluster -f - << EOF
---
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: ${CLUSTER_NAME}
  region: ${AWS_DEFAULT_REGION}
  version: "${K8S_VERSION}"
  tags:
    karpenter.sh/discovery: ${CLUSTER_NAME}
fargateProfiles:
  - name: karpenter
    selectors:
    - namespace: "${KARPENTER_NAMESPACE}"
iam:
  withOIDC: true
EOF
