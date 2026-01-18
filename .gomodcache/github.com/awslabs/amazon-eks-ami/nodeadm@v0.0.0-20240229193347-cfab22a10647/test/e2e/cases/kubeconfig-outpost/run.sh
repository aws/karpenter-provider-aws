#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source /helpers.sh

mock::imds
wait::dbus-ready

mock::kubelet 1.28.0
nodeadm init --skip run --config-source file://config.yaml
assert::file-contains /etc/hosts $'127.0.0.1\tlocalhost'
assert::file-contains /etc/hosts $'::1\tlocalhost'
