#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source /helpers.sh

mock::imds
wait::dbus-ready

mock::kubelet 1.26.0

nodeadm init --skip run --config-source file://config.yaml

assert::json-files-equal /etc/eks/image-credential-provider/config.json expected-image-credential-provider-config-126.json

mock::kubelet 1.27.0

nodeadm init --skip run --config-source file://config.yaml

assert::json-files-equal /etc/eks/image-credential-provider/config.json expected-image-credential-provider-config-127.json
