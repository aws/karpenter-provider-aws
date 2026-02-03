#!/usr/bin/env bash
set -euo pipefail

compatibilitymatrix() {
    # versionCount is the number of K8s versions to display in the compatibility matrix
    versionCount=7
    go run hack/docs/version_compatibility_gen/main.go hack/docs/compatibilitymatrix_gen/compatibility.yaml "$(git describe --exact-match --tags || echo "no tag")"
    go run hack/docs/compatibilitymatrix_gen/main.go website/content/en/preview/upgrading/compatibility.md hack/docs/compatibilitymatrix_gen/compatibility.yaml $versionCount
}

CONTROLLER_RUNTIME_DIR=$(go list -m -f '{{ .Dir }}' sigs.k8s.io/controller-runtime)
AWS_SDK_GO_PROMETHEUS_DIR=$(go list -m -f '{{ .Dir }}' github.com/jonathan-innis/aws-sdk-go-prometheus)
OPERATORPKG_DIR=$(go list -m -f '{{ .Dir }}' github.com/awslabs/operatorpkg)

compatibilitymatrix
go run hack/docs/metrics_gen/main.go pkg/ "${KARPENTER_CORE_DIR}/pkg" "${CONTROLLER_RUNTIME_DIR}/pkg" "${AWS_SDK_GO_PROMETHEUS_DIR}" "${OPERATORPKG_DIR}" website/content/en/preview/reference/metrics.md
go run hack/docs/instancetypes_gen/main.go website/content/en/preview/reference/instance-types.md
go run hack/docs/configuration_gen/main.go website/content/en/preview/reference/settings.md
cd charts/karpenter && go tool helm-docs
