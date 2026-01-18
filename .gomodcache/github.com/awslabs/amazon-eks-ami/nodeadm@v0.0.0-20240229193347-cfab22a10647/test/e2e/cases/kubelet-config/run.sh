#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source /helpers.sh

mock::imds
mock::kubelet 1.27.0
wait::dbus-ready

for config in config.*; do
  nodeadm init --skip run --config-source file://${config}
  assert::json-files-equal /etc/kubernetes/kubelet/config.json expected-kubelet-config.json
done
