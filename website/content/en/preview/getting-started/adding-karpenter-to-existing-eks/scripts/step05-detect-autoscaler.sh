kubectl -n kube-system get deploy cluster-autoscaler 2>&1 | grep -v "NotFound"
kubectl -n kube-system get deploy karpenter 2>&1 | grep -v "NotFound"
