#!/usr/bin/env bash
set -euo pipefail

K8S_VERSION="${K8S_VERSION:="1.34.x"}"
KUBEBUILDER_ASSETS="${KUBEBUILDER_ASSETS:-/usr/local/kubebuilder/bin}"


main() {
    tools
    kubebuilder
}

tools() {
    go install github.com/google/go-licenses@v1.6.0
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
    go install github.com/google/ko@v0.18.1
    go install github.com/mikefarah/yq/v4@v4.52.5
    go install github.com/norwoodj/helm-docs/cmd/helm-docs@v1.14.2
    go install sigs.k8s.io/controller-runtime/tools/setup-envtest@b9bccfd419149d26d14130887a5e5819e4a3b2be
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.21.0
    go install github.com/sigstore/cosign/v2/cmd/cosign@v2.6.2
    go install -tags extended github.com/gohugoio/hugo@v0.110.0
    go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
    go install github.com/onsi/ginkgo/v2/ginkgo@v2.32.0
    go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
    go install github.com/mattn/goveralls@v0.0.12
    go install github.com/google/go-containerregistry/cmd/crane@v0.21.3
    go install oras.land/oras/cmd/oras@v1.2.3 # Pin to this version since the latest version requires go 1.25

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
}

main "$@"
