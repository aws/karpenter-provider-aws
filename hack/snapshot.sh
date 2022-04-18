#!/bin/bash -e

SNAPSHOT_TAG=$(git rev-parse HEAD)
RELEASE_REPO=${RELEASE_REPO:-public.ecr.aws/z4v8y7u8/}
source release_common.sh

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
buildImages $HELM_CHART_VERSION
publishHelmChart
