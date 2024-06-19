[settings.kubernetes]
kube-api-qps = 30
[settings.kubernetes.node-taints]
"node.cilium.io/agent-not-ready" = ["true:NoExecute"]
