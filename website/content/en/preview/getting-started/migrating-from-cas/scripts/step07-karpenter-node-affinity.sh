cat << EOF > karpenter-node-affinity.yaml
controller:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
              - key: karpenter.sh/nodepool
                operator: DoesNotExist
              - key: eks.amazonaws.com/nodegroup
                operator: In
                values:
                  - "\${NODEGROUP}"
EOF