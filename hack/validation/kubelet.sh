# Kubelet Validation 

# kubelet.kubeReserved and kubelet.systemReserved map values may be either a resource.Quantity
# (e.g. "512Mi", "200m") or a CEL expression evaluated per instance type (e.g. "(11 * max_pods + 255) * 1048576").
# Because CEL expression syntax is arbitrary, no quantity-only CRD pattern is imposed here — doing so would
# reject valid expressions at admission before the controller can interpret them. Instead, the nodeclass
# validation controller disambiguates each value (resource.ParseQuantity first, else compile as CEL) and
# surfaces any syntax/evaluation errors on the NodeClass status. Supported functions and variables are gated
# by the CEL environment in pkg/cel/environment.go, not by this schema.
# Quantity: https://github.com/kubernetes/apimachinery/blob/d82afe1e363acae0e8c0953b1bc230d65fdb50e2/pkg/api/resource/quantity.go#L100

# The regular expression is a validation for kubelet.evictionHard and kubelet.evictionSoft are percentage or a resource.Quantity
# Quantity: https://github.com/kubernetes/apimachinery/blob/d82afe1e363acae0e8c0953b1bc230d65fdb50e2/pkg/api/resource/quantity.go#L100
# EC2NodeClass Validation:
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.kubelet.properties.evictionHard.additionalProperties.pattern = "^((\d{1,2}(\.\d{1,2})?|100(\.0{1,2})?)%||(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?)$"' -i pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml 
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.kubelet.properties.evictionSoft.additionalProperties.pattern = "^((\d{1,2}(\.\d{1,2})?|100(\.0{1,2})?)%||(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?)$"' -i pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml 
