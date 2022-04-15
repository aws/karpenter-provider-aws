#!/bin/bash -e

SNAPSHOT_TAG=$(git rev-parse HEAD)

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
source "${SCRIPT_DIR}/release_common.sh"

if [ -z "$CLOUD_PROVIDER" ]; then
    echo "CLOUD_PROVIDER environment variable is not set: 'export CLOUD_PROVIDER=aws'"
    exit 1
fi

buildImage() {
    CONTROLLER_DIGEST=$(GOFLAGS=${GOFLAGS} KO_DOCKER_REPO=${RELEASE_REPO} ko publish -B -t ${RELEASE_VERSION} ${RELEASE_PLATFORM} ./cmd/controller)
    WEBHOOK_DIGEST=$(GOFLAGS=${GOFLAGS} KO_DOCKER_REPO=${RELEASE_REPO} ko publish -B -t ${RELEASE_VERSION} ${RELEASE_PLATFORM} ./cmd/webhook)
    yq e -i ".controller.image = \"${CONTROLLER_DIGEST}\"" charts/karpenter/values.yaml
    yq e -i ".webhook.image = \"${WEBHOOK_DIGEST}\"" charts/karpenter/values.yaml
    yq e -i ".appVersion = \"${RELEASE_VERSION#v}\"" charts/karpenter/Chart.yaml
    yq e -i ".version = \"${HELM_CHART_VERSION#v}\"" charts/karpenter/Chart.yaml
}

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

authenticate
buildImage
publishHelmChart
