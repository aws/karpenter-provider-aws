# Updating the set of required items in our status conditions so that we support older versions of the condition

yq eval 'del(.spec.versions[0].schema.openAPIV3Schema.properties.status.properties.conditions.items.properties.reason.minLength)' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.status.properties.conditions.items.properties.reason.pattern = "^([A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?|)$"' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.status.properties.conditions.items.required = ["lastTransitionTime","status","type"]' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml