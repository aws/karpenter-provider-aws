#!/usr/bin/env bash
set -euo pipefail

compatibilitymatrix() {
    go run hack/docs/version_compatibility.go hack/docs/compatibility-karpenter.yaml "$(git describe --exact-match --tags || echo "no tag")"
    go run hack/docs/compatibilitymetrix_gen_docs.go website/content/en/preview/upgrading/compatibility.md hack/docs/compatibility-karpenter.yaml 6
}


compatibilitymatrix
go run hack/docs/metrics_gen_docs.go pkg/ "${KARPENTER_CORE_DIR}/pkg" website/content/en/preview/reference/metrics.md
go run hack/docs/instancetypes_gen_docs.go website/content/en/preview/reference/instance-types.md
go run hack/docs/configuration_gen_docs.go website/content/en/preview/reference/settings.md
crd-ref-docs --source-path "${KARPENTER_CORE_DIR}/pkg/apis/v1beta1" --config ./hack/docs/apis/config.yaml --output-path website/content/en/preview/reference/apis/core.md --renderer markdown
crd-ref-docs --source-path "pkg/apis/v1beta1" --config ./hack/docs/apis/config.yaml --output-path website/content/en/preview/reference/apis/aws.md --renderer markdown
cd charts/karpenter && helm-docs