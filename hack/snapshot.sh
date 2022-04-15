#!/bin/bash -e

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
SNAPSHOT_TAG=$(git rev-parse HEAD)

source "${SCRIPT_DIR}/release_common.sh"

publishHelmChart() {
    (
        HELM_CHART_FILE_NAME="karpenter-${HELM_CHART_VERSION}.tgz"

        cd charts
        helm lint karpenter
        helm package karpenter --version $HELM_CHART_VERSION
        helm push "${HELM_CHART_FILE_NAME}" "oci://${PUBLIC_ECR_REGISTRY_ALIAS}"
        rm "${HELM_CHART_FILE_NAME}"
    )
}

requireCloudProvider
authenticate
buildImage $HELM_CHART_VERSION
publishHelmChart
