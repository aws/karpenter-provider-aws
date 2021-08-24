#!/bin/bash

set -eu -o pipefail

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

main() {
    tools
    kubebuilder
}

tools() {
    cd tools
    go mod tidy
    GO111MODULE=on cat tools.go | grep _ | awk -F'"' '{print $2}' | xargs -tI % go install %
}

kubebuilder() {
    KUBEBUILDER_ASSETS="/usr/local/kubebuilder"
    sudo rm -rf $KUBEBUILDER_ASSETS
    sudo mkdir -p $KUBEBUILDER_ASSETS
    sudo mv "$(setup-envtest use -p path)" $KUBEBUILDER_ASSETS/bin
    find $KUBEBUILDER_ASSETS
}

main "$@"
