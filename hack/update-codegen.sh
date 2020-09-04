#!/bin/bash
set -eu -o pipefail


# Generate API Deep Copy
controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./pkg/apis/..."
# Generate CRDs
controller-gen crd:trivialVersions=false paths="./pkg/apis/..." "output:crd:artifacts:config=config/crd/bases"

# TODO Fix Me, doesn't generate anything
controller-gen rbac:roleName=karpenter paths="./pkg/controllers/..." output:stdout

# TODO Fix Me, doesn't generate anything
controller-gen webhook paths="./pkg/controllers/..."

# TODO Fix Me, this is broken into above generators
# controller-gen \
#     object:headerFile="hack/boilerplate.go.txt" \
#     webhook \
#     crd:trivialVersions=false \
#     rbac:roleName=manager-role \
#     paths="./pkg/apis/..." \
#     "output:crd:artifacts:config=config/crd/bases"

# TODO Fix Me, creates empty clients
# bash -e $GOPATH/pkg/mod/k8s.io/code-generator@v0.18.6/generate-groups.sh \
#     all \
#     "github.com/ellistarn/karpenter/pkg/client" \
#     "github.com/ellistarn/karpenter/pkg/apis" \
#     "autoscaling:v1alpha1" \
#     --go-header-file ./hack/boilerplate.go.txt -v2

./hack/boilerplate.sh
