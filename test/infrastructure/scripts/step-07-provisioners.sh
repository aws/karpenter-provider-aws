# Provisioner for KIT Guest Clusters
cat <<EOF | kubectl apply -f -
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  requirements:
  - key: kit.k8s.sh/app
    operator: Exists
  - key: kit.k8s.sh/control-plane-name
    operator: Exists
  - key: "kubernetes.io/arch"
    operator: NotIn
    values: ["amd64"]
  ttlSecondsAfterEmpty: 180
  provider:
    instanceProfile: KarpenterGuestClusterNodeInstanceProfile-${CLUSTER_NAME}
    tags:
      kit.aws/substrate: ${CLUSTER_NAME}
    subnetSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
    securityGroupSelector:
      kubernetes.io/cluster/${CLUSTER_NAME}: owned
EOF

# Provisioner for Tekton Pods
cat << EOF | kubectl apply -f -
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: tekton-provisioner
spec:
  ttlSecondsAfterEmpty: 600
  requirements:
  - key: "kubernetes.io/arch"
    operator: In
    values: ["amd64"]
  provider:
    subnetSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
    securityGroupSelector:
      kubernetes.io/cluster/${CLUSTER_NAME}: owned
EOF
