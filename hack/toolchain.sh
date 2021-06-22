#!/usr/bin/env bash

set -eu -o pipefail

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

main() {
    tools
    if ! command -v kubebuilder &>/dev/null; then
        install_kubebuilder
    fi
}

tools() {
    cd tools
    GO111MODULE=on cat tools.go | grep _ | awk -F'"' '{print $2}' | xargs -tI % go install %
}

install_kubebuilder() {
    os=$(go env GOOS)
    arch=$(go env GOARCH)
    curl -L "https://go.kubebuilder.io/dl/2.3.1/${os}/${arch}" | tar -xz -C $TEMP_DIR
    sudo mkdir -p /usr/local/kubebuilder/bin/
    sudo mv $TEMP_DIR/kubebuilder_2.3.1_${os}_${arch}/bin/* /usr/local/kubebuilder/bin/
    echo 'Add kubebuilder to your path `export PATH=$PATH:/usr/local/kubebuilder/bin/`'
}

main "$@"
