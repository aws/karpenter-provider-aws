#!/bin/bash
set -eu -o pipefail

controllergen=$GOPATH/bin/controller-gen
generategroups=$GOPATH/pkg/mod/k8s.io/code-generator@v0.17.2/generate-groups.sh
chmod +x $generategroups

# Generate Deepcopy
$controllergen \
    object:headerFile="hack/boilerplate.go.txt" \
    paths="./pkg/apis/..."

# Generate CRDs
$controllergen \
    crd:trivialVersions=false \
    rbac:roleName=manager-role \
    webhook paths="./pkg/apis/..." \
    "output:crd:artifacts:config=config/crd/bases"

# Generate Clients
$generategroups \
    all \
    "github.com/ellistarn/karpenter/pkg/client" \
    "github.com/ellistarn/karpenter/pkg/apis" \
    "autoscaling:v1alpha1" \
    --go-header-file ./hack/boilerplate.go.txt

# Add Boilerplate
./hack/boilerplate.sh
