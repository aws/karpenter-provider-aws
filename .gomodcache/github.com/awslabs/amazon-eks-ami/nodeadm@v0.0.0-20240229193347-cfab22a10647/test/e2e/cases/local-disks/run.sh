#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source /helpers.sh

mock::imds
wait::dbus-ready
mock::kubelet 1.29.0

mock::setup-local-disks

nodeadm init --daemon="" --config-source file://config.yaml

assert::file-contains /var/log/setup-local-disks.log 'raid0'
