[settings.kubernetes]
kube-api-qps = 30
[settings.kubernetes.node-taints]
"node.cilium.io/agent-not-ready" = ["true:NoExecute"]
[settings.kubernetes.eviction-soft]
"memory.available" = "300Mi"