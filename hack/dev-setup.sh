#!/bin/bash

set -eu -o pipefail

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

main() {
    golangcilint
    kubebuilder
}


golangcilint() {
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.31.0
}

kubebuilder() {
    os=$(go env GOOS)
    arch=$(go env GOARCH)
    curl -L "https://go.kubebuilder.io/dl/2.3.1/${os}/${arch}" | tar -xz -C $TEMP_DIR
    sudo mkdir -p /usr/local/kubebuilder/bin/
	sudo mv $TEMP_DIR/kubebuilder_2.3.1_${os}_${arch}/bin/* /usr/local/kubebuilder/bin/
    echo 'Add kubebuilder to your path `export PATH=$PATH:/usr/local/kubebuilder/bin/`'
}

main "$@"
