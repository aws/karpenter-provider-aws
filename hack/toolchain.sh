#!/usr/bin/env bash
set -euo pipefail

K8S_VERSION="${K8S_VERSION:="1.23.x"}"
KUBEBUILDER_ASSETS="${KUBEBUILDER_ASSETS:="${HOME}/.kubebuilder/bin"}"

main() {
    tools
    kubebuilder
}

tools() {
    go install github.com/google/go-licenses@v1.6.0
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.1
    go install github.com/google/ko@latest
    go install github.com/mikefarah/yq/v4@v4.30.8
    go install github.com/norwoodj/helm-docs/cmd/helm-docs@v1.11.0
    go install sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20230213155330-c98b7fd9183a
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.11.3
    go install github.com/sigstore/cosign/cmd/cosign@v1.13.1
    go install -tags extended github.com/gohugoio/hugo@v0.110.0
    go install golang.org/x/vuln/cmd/govulncheck@latest
    go install github.com/onsi/ginkgo/v2/ginkgo@latest

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
