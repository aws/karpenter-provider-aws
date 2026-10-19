#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
# shellcheck source=hack/release/common.sh
source "${SCRIPT_DIR}/release/common.sh"

compatibilitymatrix() {
    # versionCount is the number of K8s versions to display in the compatibility matrix
    versionCount=7
    tag="$(git describe --exact-match --tags || echo "no tag")"
    # Patch releases share the compatibility entry of their minor version
    if ! isPatchRelease "${tag#v}"; then
        go run hack/docs/version_compatibility_gen/main.go hack/docs/compatibilitymatrix_gen/compatibility.yaml "${tag}"
    fi
    go run hack/docs/compatibilitymatrix_gen/main.go website/content/en/preview/upgrading/compatibility.md hack/docs/compatibilitymatrix_gen/compatibility.yaml $versionCount
}

# Ensure modules are downloaded so that go list -m -f '{{ .Dir }}' can resolve paths.
go mod download

if [[ -z "${KARPENTER_CORE_DIR:-}" ]]; then
    KARPENTER_CORE_DIR=$(go list -m -f '{{ .Dir }}' sigs.k8s.io/karpenter)
fi
CONTROLLER_RUNTIME_DIR=$(go list -m -f '{{ .Dir }}' sigs.k8s.io/controller-runtime)
AWS_SDK_GO_PROMETHEUS_DIR=$(go list -m -f '{{ .Dir }}' github.com/jonathan-innis/aws-sdk-go-prometheus)
OPERATORPKG_DIR=$(go list -m -f '{{ .Dir }}' github.com/awslabs/operatorpkg)

compatibilitymatrix
go run hack/docs/metrics_gen/main.go pkg/ "${KARPENTER_CORE_DIR}/pkg" "${CONTROLLER_RUNTIME_DIR}/pkg" "${AWS_SDK_GO_PROMETHEUS_DIR}" "${OPERATORPKG_DIR}" website/content/en/preview/reference/metrics.md
go run hack/docs/instancetypes_gen/main.go website/content/en/preview/reference/instance-types.md
go run hack/docs/configuration_gen/main.go website/content/en/preview/reference/settings.md
go run hack/docs/repairpolicies_gen/main.go website/content/en/preview/concepts/disruption.md
go run hack/docs/annotations_gen/main.go website/content/en/preview/reference/annotations.md
cd charts/karpenter && helm-docs
