#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source /helpers.sh

mock::imds
wait::dbus-ready

mock::kubelet 1.28.0
nodeadm init --skip run --config-source file://config.yaml
assert::file-contains /etc/eks/kubelet/environment '--pod-infra-container-image=602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/pause:3.5'

mock::kubelet 1.29.0
nodeadm init --skip run --config-source file://config.yaml
assert::file-not-contains /etc/eks/kubelet/environment 'pod-infra-container-image'
