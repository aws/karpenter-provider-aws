# Labels Validation 

# NodePool Validation:
# checking for restricted labels while filtering out well-known labels
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.metadata.properties.labels.maxProperties = 100' -i pkg/apis/crds/karpenter.sh_nodepools.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.metadata.properties.labels.x-kubernetes-validations += [
    {"message": "label domain \"kubernetes.io\" is restricted", "rule": "self.all(x, x in [\"beta.kubernetes.io/instance-type\", \"failure-domain.beta.kubernetes.io/region\",  \"beta.kubernetes.io/os\", \"beta.kubernetes.io/arch\", \"failure-domain.beta.kubernetes.io/zone\", \"topology.kubernetes.io/zone\", \"topology.kubernetes.io/region\", \"kubernetes.io/arch\", \"kubernetes.io/os\", \"node.kubernetes.io/windows-build\"] || x.find(\"^([^/]+)\").endsWith(\"node.kubernetes.io\") || x.find(\"^([^/]+)\").endsWith(\"node-restriction.kubernetes.io\") || !x.find(\"^([^/]+)\").endsWith(\"kubernetes.io\"))"},
    {"message": "label domain \"k8s.io\" is restricted", "rule": "self.all(x, x.find(\"^([^/]+)\").endsWith(\"kops.k8s.io\") || !x.find(\"^([^/]+)\").endsWith(\"k8s.io\"))"},
    {"message": "label domain \"karpenter.sh\" is restricted", "rule": "self.all(x, x in [\"karpenter.sh/capacity-type\", \"karpenter.sh/nodepool\"] || !x.find(\"^([^/]+)\").endsWith(\"karpenter.sh\"))"},
    {"message": "label \"karpenter.sh/nodepool\" is restricted", "rule": "self.all(x, x != \"karpenter.sh/nodepool\")"},
    {"message": "label \"kubernetes.io/hostname\" is restricted", "rule": "self.all(x, x != \"kubernetes.io/hostname\")"}]' -i pkg/apis/crds/karpenter.sh_nodepools.yaml
# Vaild requirement value check
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.metadata.properties.labels.additionalProperties.maxLength = 63' -i pkg/apis/crds/karpenter.sh_nodepools.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.metadata.properties.labels.additionalProperties.pattern  = "^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$"' -i pkg/apis/crds/karpenter.sh_nodepools.yaml