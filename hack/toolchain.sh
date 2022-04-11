#!/usr/bin/env bash
set -euo pipefail

K8S_VERSION="${K8S_VERSION:="1.22.x"}"
KUBEBUILDER_ASSETS="${KUBEBUILDER_ASSETS:="${HOME}/.kubebuilder/bin"}"

main() {
    tools
    kubebuilder
}

tools() {
    go install github.com/mitchellh/golicense@v0.2.0
    go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.3.1
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.41.1
    go install github.com/google/ko@v0.11.2
    go install github.com/mikefarah/yq/v4@v4.16.1
    go install github.com/mitchellh/golicense@v0.2.0
    go install github.com/norwoodj/helm-docs/cmd/helm-docs@v1.7.0
    go install github.com/onsi/ginkgo/ginkgo@v1.16.5
    go install sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20220113220429-45b13b951f77
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0
    go install github.com/sigstore/cosign/cmd/cosign@v1.5.1
    go install github.com/gohugoio/hugo@v0.92.1+extended

    if ! echo "$PATH" | grep -q "${GOPATH:-undefined}/bin\|$HOME/go/bin"; then
        echo "Go workspace's \"bin\" directory is not in PATH. Run 'export PATH=\"\$PATH:\${GOPATH:-\$HOME/go}/bin\"'."
    fi
}

kubebuilder() {
    mkdir -p $KUBEBUILDER_ASSETS
    arch=$(go env GOARCH)
    ## Kubebuilder does not support darwin/arm64, so use amd64 through Rosetta instead
    if [[ $(go env GOOS) == "darwin" ]] && [[ $(go env GOARCH) == "arm64" ]]; then
        arch="amd64"
    fi
    ln -sf $(setup-envtest use -p path "${K8S_VERSION}" --arch="${arch}" --bin-dir="${KUBEBUILDER_ASSETS}")/* ${KUBEBUILDER_ASSETS}
    find $KUBEBUILDER_ASSETS
}

main "$@"
