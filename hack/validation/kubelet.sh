# Kubelet Validation 

# The regular expression adds validation for kubelet.kubeReserved and kubelet.systemReserved values of the map are resource.Quantity
# Quantity: https://github.com/kubernetes/apimachinery/blob/d82afe1e363acae0e8c0953b1bc230d65fdb50e2/pkg/api/resource/quantity.go#L100
# EC2NodeClass Validation:
go tool -modfile=go.tools.mod yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.kubelet.properties.kubeReserved.additionalProperties.pattern = "^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$"' -i pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml
go tool -modfile=go.tools.mod yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.kubelet.properties.systemReserved.additionalProperties.pattern = "^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$"' -i pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml 

# The regular expression is a validation for kubelet.evictionHard and kubelet.evictionSoft are percentage or a resource.Quantity
# Quantity: https://github.com/kubernetes/apimachinery/blob/d82afe1e363acae0e8c0953b1bc230d65fdb50e2/pkg/api/resource/quantity.go#L100
# EC2NodeClass Validation:
go tool -modfile=go.tools.mod yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.kubelet.properties.evictionHard.additionalProperties.pattern = "^((\d{1,2}(\.\d{1,2})?|100(\.0{1,2})?)%||(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?)$"' -i pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml 
go tool -modfile=go.tools.mod yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.kubelet.properties.evictionSoft.additionalProperties.pattern = "^((\d{1,2}(\.\d{1,2})?|100(\.0{1,2})?)%||(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?)$"' -i pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml 
