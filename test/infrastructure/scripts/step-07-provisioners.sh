# General purpose provisioner for test execution
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
  ttlSecondsAfterEmpty: 180
  provider:
    subnetSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
    securityGroupSelector:
      kubernetes.io/cluster/${CLUSTER_NAME}: owned
EOF
