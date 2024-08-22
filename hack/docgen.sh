#!/usr/bin/env bash
set -euo pipefail

compatibilitymatrix() {
    # versionCount is the number of K8s versions to display in the compatibility matrix
    versionCount=7
    go run hack/docs/version_compatibility_gen/main.go hack/docs/compatibilitymatrix_gen/compatibility.yaml "$(git describe --exact-match --tags || echo "no tag")"
    go run hack/docs/compatibilitymatrix_gen/main.go website/content/en/preview/upgrading/compatibility.md hack/docs/compatibilitymatrix_gen/compatibility.yaml $versionCount
}


compatibilitymatrix
go run hack/docs/metrics_gen/main.go pkg/ "${KARPENTER_CORE_DIR}/pkg" website/content/en/preview/reference/metrics.md
go run hack/docs/instancetypes_gen/main.go website/content/en/preview/reference/instance-types.md
go run hack/docs/configuration_gen/main.go website/content/en/preview/reference/settings.md
cd charts/karpenter && helm-docs
