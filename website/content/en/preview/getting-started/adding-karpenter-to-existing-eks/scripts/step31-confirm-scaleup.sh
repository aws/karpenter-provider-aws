# NodeClaim moved to READY=True
kubectl get nodeclaims -o wide
# All 5 pods scheduled onto the new node, not on system nodes
kubectl get pods -l app=inflate -o wide
