#!/bin/bash -e
SNAPSHOT_TAG=$(git rev-parse HEAD)
CURRENT_MAJOR_VERSION="0"
PUBLIC_ECR_REGISTRY_ALIAS="public.ecr.aws/z4v8y7u8/"
RELEASE_REPO=${RELEASE_REPO:-"${PUBLIC_ECR_REGISTRY_ALIAS}"}
RELEASE_VERSION=${RELEASE_VERSION:-"${SNAPSHOT_TAG}"}
RELEASE_PLATFORM="--platform=linux/amd64,linux/arm64"

if [ -z "$CLOUD_PROVIDER" ]; then
    echo "CLOUD_PROVIDER environment variable is not set: 'export CLOUD_PROVIDER=aws'"
    exit 1
fi

authenticate() {
  aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin ${RELEASE_REPO}
}

buildImage() {
    CONTROLLER_DIGEST=$(GOFLAGS=${GOFLAGS} KO_DOCKER_REPO=${RELEASE_REPO} ko publish -B -t ${RELEASE_VERSION} ${RELEASE_PLATFORM} ./cmd/controller)
    WEBHOOK_DIGEST=$(GOFLAGS=${GOFLAGS} KO_DOCKER_REPO=${RELEASE_REPO} ko publish -B -t ${RELEASE_VERSION} ${RELEASE_PLATFORM} ./cmd/webhook)
}

publishHelmChart() {
    (
        HELM_CHART_VERSION="v${CURRENT_MAJOR_VERSION}-${SNAPSHOT_TAG}"
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
