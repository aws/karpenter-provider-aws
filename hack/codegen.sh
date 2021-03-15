#!/bin/bash

controller-gen \
    object:headerFile="hack/boilerplate.go.txt" \
    webhook \
    crd:trivialVersions=false \
    paths="./pkg/..." \
    output:crd:artifacts:config=config \
    output:webhook:artifacts:config=config

./hack/boilerplate.sh

mv config/provisioning.karpenter.sh_provisioners.yaml config/provisioner-crd.yaml
mv config/manifests.yaml config/webhooks.yaml

# Fixup Webhook code generation. controller-gen assumes using kustomize; we do this instead
yq e -i '.metadata.name = "karpenter-" + .metadata.name' ./config/webhooks.yaml
yq e -i '.metadata.annotations = { "cert-manager.io/inject-ca-from" : "karpenter/karpenter-serving-cert" }' ./config/webhooks.yaml
yq e -i '.webhooks[].clientConfig.service.name = "karpenter-webhook-service"' ./config/webhooks.yaml
yq e -i '.webhooks[].clientConfig.service.namespace = "karpenter"' ./config/webhooks.yaml
yq e -i 'del(.webhooks[].admissionReviewVersions[0])' ./config/webhooks.yaml

# Hack to remove v1.AdmissionReview until https://github.com/kubernetes-sigs/controller-runtime/issues/1161 is fixed
perl -pi -e 's/^  - v1$$//g' config/webhooks.yaml

# CRDs don't currently jive with VolatileTime, which has an Any type.
perl -pi -e 's/Any/string/g' config/provisioner-crd.yaml
