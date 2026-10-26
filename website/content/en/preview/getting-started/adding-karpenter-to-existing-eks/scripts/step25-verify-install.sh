# 1. Both pods Running on different zones
kubectl get pods -n "${KARPENTER_NAMESPACE}" -l app.kubernetes.io/name=karpenter -o wide

# 2. CRDs registered
kubectl get crds | grep karpenter

# 3. Service account has the IRSA annotation
kubectl get sa -n "${KARPENTER_NAMESPACE}" karpenter -o jsonpath='{.metadata.annotations.eks\.amazonaws\.com/role-arn}'; echo

# 4. Controller acquired leader lease and started its sub-controllers
kubectl logs -n "${KARPENTER_NAMESPACE}" -l app.kubernetes.io/name=karpenter --tail=200 \
  | grep -E '"message":"(starting server|Successfully acquired lease|Starting Controller)"' \
  | head -5
