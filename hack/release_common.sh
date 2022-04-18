#!/bin/bash -e

CURRENT_MAJOR_VERSION="0"
HELM_CHART_VERSION="v${CURRENT_MAJOR_VERSION}-${SNAPSHOT_TAG}"
RELEASE_VERSION=${RELEASE_VERSION:-"${SNAPSHOT_TAG}"}
RELEASE_PLATFORM="--platform=linux/amd64,linux/arm64"

requireCloudProvider(){
  if [ -z "$CLOUD_PROVIDER" ]; then
      echo "CLOUD_PROVIDER environment variable is not set: 'export CLOUD_PROVIDER=aws'"
      exit 1
  fi
}

authenticate() {
  aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin ${RELEASE_REPO}
}


buildImages() {
    HELM_CHART_VERSION=$1
    CONTROLLER_DIGEST=$(GOFLAGS=${GOFLAGS} KO_DOCKER_REPO=${RELEASE_REPO} ko publish -B -t ${RELEASE_VERSION} ${RELEASE_PLATFORM} ./cmd/controller)
    WEBHOOK_DIGEST=$(GOFLAGS=${GOFLAGS} KO_DOCKER_REPO=${RELEASE_REPO} ko publish -B -t ${RELEASE_VERSION} ${RELEASE_PLATFORM} ./cmd/webhook)
    yq e -i ".controller.image = \"${CONTROLLER_DIGEST}\"" charts/karpenter/values.yaml
    yq e -i ".webhook.image = \"${WEBHOOK_DIGEST}\"" charts/karpenter/values.yaml
    yq e -i ".appVersion = \"${RELEASE_VERSION#v}\"" charts/karpenter/Chart.yaml
    yq e -i ".version = \"${HELM_CHART_VERSION#v}\"" charts/karpenter/Chart.yaml
}
