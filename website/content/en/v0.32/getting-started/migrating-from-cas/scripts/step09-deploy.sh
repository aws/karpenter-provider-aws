kubectl create namespace "${KARPENTER_NAMESPACE}" || true
kubectl create -f \
    https://raw.githubusercontent.com/aws/karpenter-provider-aws/${KARPENTER_VERSION}/pkg/apis/crds/karpenter.sh_provisioners.yaml
kubectl create -f \
    https://raw.githubusercontent.com/aws/karpenter-provider-aws/${KARPENTER_VERSION}/pkg/apis/crds/karpenter.k8s.aws_awsnodetemplates.yaml
kubectl create -f \
    https://raw.githubusercontent.com/aws/karpenter-provider-aws/${KARPENTER_VERSION}/pkg/apis/crds/karpenter.sh_machines.yaml
kubectl create -f \
    https://raw.githubusercontent.com/aws/karpenter-provider-aws/${KARPENTER_VERSION}/pkg/apis/crds/karpenter.sh_nodepools.yaml
kubectl create -f \
    https://raw.githubusercontent.com/aws/karpenter-provider-aws/${KARPENTER_VERSION}/pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml
kubectl create -f \
    https://raw.githubusercontent.com/aws/karpenter-provider-aws/${KARPENTER_VERSION}/pkg/apis/crds/karpenter.sh_nodeclaims.yaml
kubectl apply -f karpenter.yaml
