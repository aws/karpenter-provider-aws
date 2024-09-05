#!/usr/bin/env bash
set -euo pipefail

compatibilitymatrix() {
    # versionCount is the number of K8s versions to display in the compatibility matrix
    versionCount=7
    go run hack/docs/versioncompatibility/main.go hack/docs/compatibilitymatrix/compatibility-karpenter.yaml "$(git describe --exact-match --tags || echo "no tag")"
    go run hack/docs/compatibilitymatrix/main.go website/content/en/preview/upgrading/compatibility.md hack/docs/compatibilitymatrix/compatibility-karpenter.yaml $versionCount
}


compatibilitymatrix
go run hack/docs/metrics/main.go pkg/ ${KARPENTER_CORE_DIR}/pkg website/content/en/preview/reference/metrics.md
go run hack/docs/instancetypes/main.go website/content/en/preview/reference/instance-types.md
go run hack/docs/configuration/main.go website/content/en/preview/reference/settings.md
cd charts/karpenter && helm-docs
