#!/usr/bin/env bash

# Add the conversion stanza to the main chart CRD spec to enable conversion via webhook
yq eval '.spec.conversion = {"strategy": "Webhook", "webhook": {"conversionReviewVersions": ["v1beta1", "v1"], "clientConfig": {"service": {"name": "karpenter", "namespace": "kube-system", "port": 8443}}}}' -i pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml
yq eval '.spec.conversion = {"strategy": "Webhook", "webhook": {"conversionReviewVersions": ["v1beta1", "v1"], "clientConfig": {"service": {"name": "karpenter", "namespace": "kube-system", "port": 8443}}}}' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml
yq eval '.spec.conversion = {"strategy": "Webhook", "webhook": {"conversionReviewVersions": ["v1beta1", "v1"], "clientConfig": {"service": {"name": "karpenter", "namespace": "kube-system", "port": 8443}}}}' -i pkg/apis/crds/karpenter.sh_nodepools.yaml

# Update to the karpenter-crd charts
# Remove the copied over versions and conversion stanzas from CRD spec
yq eval 'del(.spec.versions)' -i charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml
yq eval 'del(.spec.conversion)' -i charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml
yq eval 'del(.spec.versions)' -i charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml
yq eval 'del(.spec.conversion)' -i charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml
yq eval 'del(.spec.versions)' -i charts/karpenter-crd/templates/karpenter.sh_nodepools.yaml
yq eval 'del(.spec.conversion)' -i charts/karpenter-crd/templates/karpenter.sh_nodepools.yaml

# Template the v1 version and the conversion strategy of the spec 
hack/mutation/ec2nodeclasses.sh
hack/mutation/nodepools.sh
hack/mutation/nodeclaims.sh