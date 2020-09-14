#!/bin/bash
set -eu -o pipefail

# Controller code generation
controller-gen \
    object:headerFile="hack/boilerplate.go.txt" \
    webhook \
    crd:trivialVersions=false \
    rbac:roleName=karpenter \
    paths="./pkg/..." \
    output:crd:artifacts:config=config/crd/bases \
    output:webhook:artifacts:config=config/webhook

# TODO Fix Me, creates empty clients
# bash -e $GOPATH/pkg/mod/k8s.io/code-generator@v0.18.6/generate-groups.sh \
#     all \
#     "github.com/ellistarn/karpenter/pkg/client" \
#     "github.com/ellistarn/karpenter/pkg/apis" \
#     "autoscaling:v1alpha1" \
#     --go-header-file ./hack/boilerplate.go.txt -v2

./hack/boilerplate.sh
