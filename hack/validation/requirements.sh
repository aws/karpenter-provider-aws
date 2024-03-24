# Requirements Validation 

# Adding validation for nodeclaim 

## checking for restricted labels while filtering out well known labels
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.requirements.items.properties.key.x-kubernetes-validations += [
    {"message": "label domain \"karpenter.k8s.aws\" is restricted", "rule": "self in [\"karpenter.k8s.aws/instance-encryption-in-transit-supported\", \"karpenter.k8s.aws/instance-category\", \"karpenter.k8s.aws/instance-hypervisor\", \"karpenter.k8s.aws/instance-family\", \"karpenter.k8s.aws/instance-generation\", \"karpenter.k8s.aws/instance-local-nvme\", \"karpenter.k8s.aws/instance-size\", \"karpenter.k8s.aws/instance-cpu\",\"karpenter.k8s.aws/instance-cpu-manufacturer\",\"karpenter.k8s.aws/instance-memory\", \"karpenter.k8s.aws/instance-ebs-bandwidth\", \"karpenter.k8s.aws/instance-network-bandwidth\", \"karpenter.k8s.aws/instance-gpu-name\", \"karpenter.k8s.aws/instance-gpu-manufacturer\", \"karpenter.k8s.aws/instance-gpu-count\", \"karpenter.k8s.aws/instance-gpu-memory\", \"karpenter.k8s.aws/instance-accelerator-name\", \"karpenter.k8s.aws/instance-accelerator-manufacturer\", \"karpenter.k8s.aws/instance-accelerator-count\"] || !self.find(\"^([^/]+)\").endsWith(\"karpenter.k8s.aws\")"}]' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml 
# # Adding validation for nodepool

# ## checking for restricted labels while filtering out well known labels
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.spec.properties.requirements.items.properties.key.x-kubernetes-validations  += [
    {"message": "label domain \"karpenter.k8s.aws\" is restricted", "rule": "self in [\"karpenter.k8s.aws/instance-encryption-in-transit-supported\", \"karpenter.k8s.aws/instance-category\", \"karpenter.k8s.aws/instance-hypervisor\", \"karpenter.k8s.aws/instance-family\", \"karpenter.k8s.aws/instance-generation\", \"karpenter.k8s.aws/instance-local-nvme\", \"karpenter.k8s.aws/instance-size\", \"karpenter.k8s.aws/instance-cpu\",\"karpenter.k8s.aws/instance-cpu-manufacturer\",\"karpenter.k8s.aws/instance-memory\", \"karpenter.k8s.aws/instance-ebs-bandwidth\", \"karpenter.k8s.aws/instance-network-bandwidth\", \"karpenter.k8s.aws/instance-gpu-name\", \"karpenter.k8s.aws/instance-gpu-manufacturer\", \"karpenter.k8s.aws/instance-gpu-count\", \"karpenter.k8s.aws/instance-gpu-memory\", \"karpenter.k8s.aws/instance-accelerator-name\", \"karpenter.k8s.aws/instance-accelerator-manufacturer\", \"karpenter.k8s.aws/instance-accelerator-count\"] || !self.find(\"^([^/]+)\").endsWith(\"karpenter.k8s.aws\")"}]' -i pkg/apis/crds/karpenter.sh_nodepools.yaml 
