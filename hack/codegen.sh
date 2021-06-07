#!/bin/bash

controller-gen \
    object:headerFile="hack/boilerplate.go.txt" \
    crd:trivialVersions=false \
    paths="./pkg/..." \
    output:crd:artifacts:config=charts/karpenter/templates
# CRDs don't currently jive with VolatileTime, which has an Any type.
perl -pi -e 's/Any/string/g' charts/karpenter/templates/provisioning.karpenter.sh_provisioners.yaml

./hack/boilerplate.sh
