#!/usr/bin/env bash

# Requirements Validation
## checking for restricted labels while filtering out well-known labels
# NodeClaim Validation:
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.requirements.items.properties.key.x-kubernetes-validations += [
    {"message": "label domain \"karpenter.kwok.sh\" is restricted", "rule": "self in [\"karpenter.kwok.sh/kwoknodeclass\", \"karpenter.kwok.sh/instance-cpu\", \"karpenter.kwok.sh/instance-memory\", \"karpenter.kwok.sh/instance-family\", \"karpenter.kwok.sh/instance-size\"] || !self.find(\"^([^/]+)\").endsWith(\"karpenter.kwok.sh\")"}]' -i kwok/charts/crds/karpenter.sh_nodeclaims.yaml

# NodePool Validation:
yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.spec.properties.requirements.items.properties.key.x-kubernetes-validations  += [
    {"message": "label domain \"karpenter.kwok.sh\" is restricted", "rule": "self in [\"karpenter.kwok.sh/kwoknodeclass\", \"karpenter.kwok.sh/instance-cpu\", \"karpenter.kwok.sh/instance-memory\", \"karpenter.kwok.sh/instance-family\", \"karpenter.kwok.sh/instance-size\"] || !self.find(\"^([^/]+)\").endsWith(\"karpenter.kwok.sh\")"}]' -i kwok/charts/crds/karpenter.sh_nodepools.yaml
