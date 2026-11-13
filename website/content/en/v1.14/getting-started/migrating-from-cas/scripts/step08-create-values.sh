cat > karpenter-values.yaml <<EOF
settings:
  clusterName: "${CLUSTER_NAME}"
  interruptionQueue: "${CLUSTER_NAME}"
serviceAccount:
  annotations:
    eks.amazonaws.com/role-arn: "arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/KarpenterControllerRole-${CLUSTER_NAME}"
controller:
  resources:
    requests:
      cpu: 1
      memory: 1Gi
    limits:
      cpu: 1
      memory: 1Gi
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
          - "${NODEGROUP}"
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - topologyKey: kubernetes.io/hostname
EOF
