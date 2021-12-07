#!/bin/bash

set -eu -o pipefail

main() {
    tools
    kubebuilder
}

tools() {
    cd tools
    go mod tidy
    GO111MODULE=on cat tools.go | grep _ | awk -F'"' '{print $2}' | xargs -tI % go install %

    if ! echo "$PATH" | grep -q "${GOPATH:-undefined}/bin\|$HOME/go/bin"; then
        echo "Go workspace's \"bin\" directory is not in PATH. Run 'export PATH=\"\$PATH:\${GOPATH:-\$HOME/go}/bin\"'."
    fi
}

kubebuilder() {
    KUBEBUILDER_ASSETS="/usr/local/kubebuilder"
    sudo rm -rf $KUBEBUILDER_ASSETS
    sudo mkdir -p $KUBEBUILDER_ASSETS
    arch=$(go env GOARCH)
    ## Kubebuilder does not support darwin/arm64, so use amd64 through Rosetta instead 
    if [[ $(go env GOOS) == "darwin" ]] && [[ $(go env GOARCH) == "arm64" ]]; then
        arch="amd64"
    fi
    sudo mv "$(setup-envtest use -p path 1.21.x --arch=${arch})" $KUBEBUILDER_ASSETS/bin
    find $KUBEBUILDER_ASSETS
}

main "$@"
