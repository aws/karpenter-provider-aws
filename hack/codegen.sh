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
yq w -i -d0 ./config/webhooks.yaml 'metadata.name' karpenter-mutating-webhook-configuration
yq w -i -d0 ./config/webhooks.yaml 'metadata.annotations[cert-manager.io/inject-ca-from]' karpenter/karpenter-serving-cert
yq w -i -d0 ./config/webhooks.yaml 'webhooks[0].clientConfig.service.name' karpenter-webhook-service
yq w -i -d0 ./config/webhooks.yaml 'webhooks[0].clientConfig.service.namespace' karpenter
yq d -i -d0 ./config/webhooks.yaml 'webhooks[0].admissionReviewVersions[0]'
yq w -i -d1 ./config/webhooks.yaml 'metadata.name' karpenter-validating-webhook-configuration
yq w -i -d1 ./config/webhooks.yaml 'metadata.annotations[cert-manager.io/inject-ca-from]' karpenter/karpenter-serving-cert
yq w -i -d1 ./config/webhooks.yaml 'webhooks[0].clientConfig.service.name' karpenter-webhook-service
yq w -i -d1 ./config/webhooks.yaml 'webhooks[0].clientConfig.service.namespace' karpenter
yq d -i -d1 ./config/webhooks.yaml 'webhooks[0].admissionReviewVersions[0]'

# Hack to remove v1.AdmissionReview until https://github.com/kubernetes-sigs/controller-runtime/issues/1161 is fixed
perl -pi -e 's/^  - v1$$//g' config/webhooks.yaml

# CRDs don't currently jive with VolatileTime, which has an Any type.
perl -pi -e 's/Any/string/g' config/provisioner-crd.yaml
