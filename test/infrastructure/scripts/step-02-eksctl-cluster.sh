cmd="create"
K8S_VERSION="1.22"
eksctl get cluster --name "${CLUSTER_NAME}" && cmd="upgrade"
eksctl ${cmd} cluster -f - <<EOF
---
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: ${CLUSTER_NAME}
  region: ${AWS_REGION}
  version: "${K8S_VERSION}"
  tags:
    karpenter.sh/discovery: ${CLUSTER_NAME}
managedNodeGroups:
  - instanceTypes:
    - m5.large
    - m5a.large
    - m6i.large
    - c5.large
    - c5a.large
    - c6i.large
    amiFamily: AmazonLinux2
    name: ${CLUSTER_NAME}-system-pool
    desiredCapacity: 2
    minSize: 2
    maxSize: 2
    taints:
      - key: CriticalAddonsOnly
        value: "true"
        effect: NoSchedule
iam:
  withOIDC: true
EOF
