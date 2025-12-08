#!/usr/bin/env bash
set -euo pipefail

K8S_VERSION="${K8S_VERSION:="1.34.x"}"
KUBEBUILDER_ASSETS="${KUBEBUILDER_ASSETS:-/usr/local/kubebuilder/bin}"

main() {
    tools
    kubebuilder
}

tools() {
    # Sync tool dependencies declared in go.mod
    go mod download

    # golangci-lint is installed separately for submodule linting
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

    if ! echo "$PATH" | grep -q "${GOPATH:-undefined}/bin\|$HOME/go/bin"; then
        echo "Go workspace's \"bin\" directory is not in PATH. Run 'export PATH=\"\$PATH:\${GOPATH:-\$HOME/go}/bin\"'."
    fi
}

kubebuilder() {
    if ! mkdir -p ${KUBEBUILDER_ASSETS}; then
      sudo mkdir -p ${KUBEBUILDER_ASSETS}
      sudo chown $(whoami) ${KUBEBUILDER_ASSETS}
    fi
    arch=$(go env GOARCH)
    ln -sf $(go tool setup-envtest use -p path "${K8S_VERSION}" --arch="${arch}" --bin-dir="${KUBEBUILDER_ASSETS}")/* ${KUBEBUILDER_ASSETS}
    find $KUBEBUILDER_ASSETS
}

main "$@"
