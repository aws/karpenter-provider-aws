[settings.kubernetes]
kube-api-qps = 30
eviction-max-pod-grace-period = 40
[settings.kubernetes.node-taints]
"node.cilium.io/agent-not-ready" = ["true:NoExecute"]
[settings.kubernetes.eviction-soft]
"memory.available" = "100Mi"
[settings.kubernetes.eviction-soft-grace-period]
"memory.available" = "30s"