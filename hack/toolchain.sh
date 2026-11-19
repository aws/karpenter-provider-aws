#!/usr/bin/env bash
set -euo pipefail

K8S_VERSION="${K8S_VERSION:="1.34.x"}"
KUBEBUILDER_ASSETS="${KUBEBUILDER_ASSETS:-/usr/local/kubebuilder/bin}"


# go_install retries `go install` so a transient proxy.golang.org / sum.golang.org error
# (e.g. "stream error: ... INTERNAL_ERROR; received from peer") doesn't fail the whole job.
# Already-downloaded modules stay in the module cache, so a retry only refetches what failed.
go_install() {
    local attempts=3 i
    for ((i = 1; i <= attempts; i++)); do
        if go install "$@"; then
            return 0
        fi
        if ((i < attempts)); then
            echo "go install $* failed (attempt ${i}/${attempts}), retrying in $((i * 10))s..." >&2
            sleep $((i * 10))
        fi
    done
    echo "go install $* failed after ${attempts} attempts" >&2
    return 1
}

main() {
    tools
    kubebuilder
}

tools() {
    go_install github.com/google/go-licenses@v1.6.0
    go_install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
    go_install github.com/google/ko@v0.18.1
    go_install github.com/mikefarah/yq/v4@v4.52.5
    go_install github.com/norwoodj/helm-docs/cmd/helm-docs@v1.14.2
    go_install sigs.k8s.io/controller-runtime/tools/setup-envtest@b9bccfd419149d26d14130887a5e5819e4a3b2be
    go_install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.21.0
    go_install github.com/sigstore/cosign/v2/cmd/cosign@v2.6.2
    go_install -tags extended github.com/gohugoio/hugo@v0.110.0
    go_install golang.org/x/vuln/cmd/govulncheck@v1.1.4
    go_install github.com/onsi/ginkgo/v2/ginkgo@v2.32.0
    go_install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
    go_install github.com/mattn/goveralls@v0.0.12
    go_install github.com/google/go-containerregistry/cmd/crane@v0.21.3
    go_install oras.land/oras/cmd/oras@v1.2.3 # Pin to this version since the latest version requires go 1.25

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
