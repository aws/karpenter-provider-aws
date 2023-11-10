eksctl create cluster -f - << EOF
---
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: ${CLUSTER_NAME}
  region: ${AWS_DEFAULT_REGION}
  version: "1.24"
  tags:
    karpenter.sh/discovery: ${CLUSTER_NAME}
fargateProfiles:
  - name: karpenter
    selectors:
    - namespace: karpenter
iam:
  withOIDC: true
EOF
