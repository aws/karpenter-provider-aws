#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source /helpers.sh

mock::imds
wait::dbus-ready

mock::kubelet 1.23.0
nodeadm init --skip run --config-source file://config.yaml
assert::files-equal /var/lib/kubelet/kubeconfig expected-kubeconfig.yaml

mock::kubelet 1.28.0
nodeadm init --skip run --config-source file://config.yaml
assert::files-equal /var/lib/kubelet/kubeconfig expected-kubeconfig.yaml
