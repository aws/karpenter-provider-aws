cat <<EOF | envsubst | kubectl apply -f -
---
apiVersion: flowcontrol.apiserver.k8s.io/v1
kind: FlowSchema
metadata:
  name: karpenter-leader-election
spec:
  distinguisherMethod:
    type: ByUser
  matchingPrecedence: 200
  priorityLevelConfiguration:
    name: leader-election
  rules:
  - resourceRules:
    - apiGroups:
      - coordination.k8s.io
      namespaces:
      - '*'
      resources:
      - leases
      verbs:
      - get
      - create
      - update
    subjects:
    - kind: ServiceAccount
      serviceAccount:
        name: karpenter
        namespace: "${KARPENTER_NAMESPACE}"

---
apiVersion: flowcontrol.apiserver.k8s.io/v1
kind: FlowSchema
metadata:
  name: karpenter-workload
spec:
  distinguisherMethod:
    type: ByUser
  matchingPrecedence: 1000
  priorityLevelConfiguration:
    name: workload-high
  rules:
  - nonResourceRules:
    - nonResourceURLs:
      - '*'
      verbs:
      - '*'
    resourceRules:
    - apiGroups:
      - '*'
      clusterScope: true
      namespaces:
      - '*'
      resources:
      - '*'
      verbs:
      - '*'
    subjects:
    - kind: ServiceAccount
      serviceAccount:
        name: karpenter
        namespace: "${KARPENTER_NAMESPACE}"
EOF
