#!/usr/bin/env bash
set -euo pipefail

K8S_VERSION="${K8S_VERSION:="1.29.x"}"
KUBEBUILDER_ASSETS="${KUBEBUILDER_ASSETS:-/usr/local/kubebuilder/bin}"


main() {
    tools
    kubebuilder
}

tools() {
    go install github.com/google/go-licenses@latest
    # asciicheck is a dependency of golangci-lint that got removed so golangci changed their go.mod to use the forked version
    # fix - https://github.com/golangci/golangci-lint/issues/6017
    # change to latest once golangci releases new version with the fix
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@main
    go install github.com/google/ko@latest
    go install github.com/mikefarah/yq/v4@latest
    go install github.com/norwoodj/helm-docs/cmd/helm-docs@latest
    go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
    go install github.com/sigstore/cosign/v2/cmd/cosign@latest
    go install -tags extended github.com/gohugoio/hugo@v0.110.0
    go install golang.org/x/vuln/cmd/govulncheck@latest
    go install github.com/onsi/ginkgo/v2/ginkgo@latest
    go install github.com/rhysd/actionlint/cmd/actionlint@latest
    go install github.com/mattn/goveralls@latest
    go install github.com/google/go-containerregistry/cmd/crane@latest
    go install oras.land/oras/cmd/oras@latest

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
    ln -sf $(setup-envtest use -p path "${K8S_VERSION}" --arch="${arch}" --bin-dir="${KUBEBUILDER_ASSETS}")/* ${KUBEBUILDER_ASSETS}
    find $KUBEBUILDER_ASSETS

    # Install latest binaries for 1.25.x (contains CEL fix)
    if [[ "${K8S_VERSION}" = "1.25.x" ]] && [[ "$OSTYPE" == "linux"* ]]; then
        for binary in 'kube-apiserver' 'kubectl'; do
            rm $KUBEBUILDER_ASSETS/$binary
            wget -P $KUBEBUILDER_ASSETS dl.k8s.io/v1.25.16/bin/linux/${arch}/${binary}
            chmod +x $KUBEBUILDER_ASSETS/$binary
        done
    fi
}

main "$@"
