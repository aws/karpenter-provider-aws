kubectl create namespace karpenter
kubectl create -f \
    https://raw.githubusercontent.com/aws/karpenter/${KARPENTER_VERSION}/charts/karpenter/crds/karpenter.sh_provisioners.yaml
kubectl apply -f karpenter.yaml
