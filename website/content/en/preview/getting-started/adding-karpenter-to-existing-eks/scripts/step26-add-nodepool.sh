cat <<EOF | envsubst | kubectl apply -f -
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: default
spec:
  role: "KarpenterNodeRole-${CLUSTER_NAME}"
  amiSelectorTerms:
    - alias: al2023@${ALIAS_VERSION}
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"
  tags:
    karpenter.sh/discovery: "${CLUSTER_NAME}"
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: default
spec:
  template:
    metadata:
      labels:
        provisioner: karpenter
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: default
      requirements:
        # Editable below — defaults shown match the upstream best-practice doc.
        - key: kubernetes.io/arch
          operator: In
          values: ["amd64"]                              # add "arm64" to allow Graviton
        - key: kubernetes.io/os
          operator: In
          values: ["linux"]
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["spot", "on-demand"]                  # drop "spot" for production-critical pools
        - key: karpenter.k8s.aws/instance-category
          operator: In
          values: ["c", "m", "r"]                        # add "g","p" for GPU, "i" for storage-optimized
          minValues: 2
        - key: karpenter.k8s.aws/instance-generation
          operator: Gt
          values: ["3"]                                  # raise to "5"+ once you've validated your workloads
        - key: karpenter.k8s.aws/instance-size
          operator: NotIn
          values: ["nano", "micro", "small", "metal"]
      expireAfter: 720h                                  # 30 days; "Never" disables expiration
      terminationGracePeriod: 24h
  disruption:
    consolidationPolicy: WhenEmptyOrUnderutilized        # or "WhenEmpty" for conservative
    consolidateAfter: 1m                                 # raise to 5m+ if your workloads thrash
    budgets:
      - nodes: "10%"                                     # max % disrupted at once; tune to blast-radius tolerance
  limits:
    cpu: "<your-cpu-cap>"                                # replace, e.g. "100"
    memory: <your-memory-cap>                            # replace, e.g. 200Gi
EOF
