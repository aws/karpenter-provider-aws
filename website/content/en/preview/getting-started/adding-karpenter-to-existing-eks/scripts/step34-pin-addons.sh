kubectl patch deployment coredns -n kube-system --patch '{
  "spec": {"template": {"spec": {"affinity": {"nodeAffinity": {
    "requiredDuringSchedulingIgnoredDuringExecution": {
      "nodeSelectorTerms": [{"matchExpressions": [{
        "key": "<your-system-label-key>",
        "operator": "In",
        "values": ["<your-system-label-value>"]
      }]}]
    }
  }}}}}}'

kubectl patch deployment metrics-server -n kube-system --patch '{
  "spec": {"template": {"spec": {"affinity": {"nodeAffinity": {
    "requiredDuringSchedulingIgnoredDuringExecution": {
      "nodeSelectorTerms": [{"matchExpressions": [{
        "key": "<your-system-label-key>",
        "operator": "In",
        "values": ["<your-system-label-value>"]
      }]}]
    }
  }}}}}}'
