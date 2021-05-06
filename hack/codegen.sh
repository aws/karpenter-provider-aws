#!/bin/bash

controller-gen \
    object:headerFile="hack/boilerplate.go.txt" \
    webhook \
    crd:trivialVersions=false \
    paths="./pkg/..." \
    output:crd:artifacts:config=charts/karpenter/templates \
    output:webhook:artifacts:config=charts/karpenter/templates

./hack/boilerplate.sh

mv charts/karpenter/templates/provisioning.karpenter.sh_provisioners.yaml charts/karpenter/templates/provisioner-crd.yaml
mv charts/karpenter/templates/manifests.yaml charts/karpenter/templates/webhooks.yaml

# Fixup Webhook code generation. controller-gen assumes using kustomize; we do this instead
yq e -i '.metadata.name = "karpenter-" + .metadata.name' charts/karpenter/templates/webhooks.yaml
yq e -i '.metadata.annotations = { "cert-manager.io/inject-ca-from" : "karpenter/karpenter-serving-cert" }' charts/karpenter/templates/webhooks.yaml
yq e -i '.webhooks[].clientConfig.service.name = "karpenter-webhook-service"' charts/karpenter/templates/webhooks.yaml
yq e -i '.webhooks[].clientConfig.service.namespace = "karpenter"' charts/karpenter/templates/webhooks.yaml
yq e -i 'del(.webhooks[].admissionReviewVersions[0])' charts/karpenter/templates/webhooks.yaml

# Hack to remove v1.AdmissionReview until https://github.com/kubernetes-sigs/controller-runtime/issues/1161 is fixed
perl -pi -e 's/^  - v1$$//g' charts/karpenter/templates/webhooks.yaml

# CRDs don't currently jive with VolatileTime, which has an Any type.
perl -pi -e 's/Any/string/g' charts/karpenter/templates/provisioner-crd.yaml
