# Block until reconciliation completes (typically <30s)
kubectl wait --for=condition=Ready ec2nodeclass/default --timeout=60s

# Now every sub-condition should be True
kubectl get ec2nodeclass default -o jsonpath='{range .status.conditions[*]}{.type}={.status} {end}'; echo

# Confirm Karpenter discovered the right subnets and SGs
kubectl get ec2nodeclass default -o jsonpath='{.status.subnets[*].zone}'; echo
kubectl get ec2nodeclass default -o jsonpath='{.status.securityGroups[*].name}'; echo

# NodePool exists and is observable
kubectl get nodepool default
