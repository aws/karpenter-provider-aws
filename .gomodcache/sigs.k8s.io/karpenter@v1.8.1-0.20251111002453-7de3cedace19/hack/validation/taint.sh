# Taints Validation
# NodeClaim Validation:
## Taint
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.taints.items.properties.key.minLength = 1' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.taints.items.properties.key.pattern = "^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*(\/))?([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$"' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.taints.items.properties.value.pattern = "^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*(\/))?([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$"' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.taints.items.properties.effect.enum += ["NoSchedule","PreferNoSchedule","NoExecute"]' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml

## Startup-Taint
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.startupTaints.items.properties.key.minLength = 1' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.startupTaints.items.properties.key.pattern = "^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*(\/))?([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$"' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.startupTaints.items.properties.value.pattern = "^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*(\/))?([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$"' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.startupTaints.items.properties.effect.enum += ["NoSchedule","PreferNoSchedule","NoExecute"]' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml

# Nodepool Validation:
## Taint
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.spec.properties.taints.items.properties.key.minLength = 1' -i pkg/apis/crds/karpenter.sh_nodepools.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.spec.properties.taints.items.properties.key.pattern = "^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*(\/))?([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$"' -i pkg/apis/crds/karpenter.sh_nodepools.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.spec.properties.taints.items.properties.value.pattern = "^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*(\/))?([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$"' -i pkg/apis/crds/karpenter.sh_nodepools.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.spec.properties.taints.items.properties.effect.enum += ["NoSchedule","PreferNoSchedule","NoExecute"]' -i pkg/apis/crds/karpenter.sh_nodepools.yaml

## Startup-Taint
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.spec.properties.startupTaints.items.properties.key.minLength = 1' -i pkg/apis/crds/karpenter.sh_nodepools.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.spec.properties.startupTaints.items.properties.key.pattern = "^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*(\/))?([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$"' -i pkg/apis/crds/karpenter.sh_nodepools.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.spec.properties.startupTaints.items.properties.value.pattern = "^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*(\/))?([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$"' -i pkg/apis/crds/karpenter.sh_nodepools.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.spec.properties.startupTaints.items.properties.effect.enum += ["NoSchedule","PreferNoSchedule","NoExecute"]' -i pkg/apis/crds/karpenter.sh_nodepools.yaml

