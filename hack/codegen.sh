#!/bin/bash

controller-gen \
    object:headerFile="hack/boilerplate.go.txt" \
    webhook \
    crd:trivialVersions=false \
    paths="./pkg/..." \
    output:crd:artifacts:config=config/templates \
    output:webhook:artifacts:config=config/templates

./hack/boilerplate.sh

mv config/templates/provisioning.karpenter.sh_provisioners.yaml config/templates/provisioner-crd.yaml
mv config/templates/manifests.yaml config/templates/webhooks.yaml

# Fixup Webhook code generation. controller-gen assumes using kustomize; we do this instead
yq e -i '.metadata.name = "karpenter-" + .metadata.name' ./config/templates/webhooks.yaml
yq e -i '.metadata.annotations = { "cert-manager.io/inject-ca-from" : "karpenter/karpenter-serving-cert" }' ./config/templates/webhooks.yaml
yq e -i '.webhooks[].clientConfig.service.name = "karpenter-webhook-service"' ./config/templates/webhooks.yaml
yq e -i '.webhooks[].clientConfig.service.namespace = "karpenter"' ./config/templates/webhooks.yaml
yq e -i 'del(.webhooks[].admissionReviewVersions[0])' ./config/templates/webhooks.yaml

# Hack to remove v1.AdmissionReview until https://github.com/kubernetes-sigs/controller-runtime/issues/1161 is fixed
perl -pi -e 's/^  - v1$$//g' config/templates/webhooks.yaml

# CRDs don't currently jive with VolatileTime, which has an Any type.
perl -pi -e 's/Any/string/g' config/templates/provisioner-crd.yaml
