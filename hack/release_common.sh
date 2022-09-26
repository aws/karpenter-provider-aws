#!/usr/bin/env bash
set -euo pipefail

CURRENT_MAJOR_VERSION="0"
PRIVATE_PULL_THROUGH_HOST="339104714817.dkr.ecr.us-east-1.amazonaws.com"
HELM_CHART_VERSION="v${CURRENT_MAJOR_VERSION}-${SNAPSHOT_TAG}"
RELEASE_VERSION=${RELEASE_VERSION:-"${SNAPSHOT_TAG}"}
RELEASE_PLATFORM="--platform=linux/amd64,linux/arm64"

# TODO restore https://reproducible-builds.org/docs/source-date-epoch/
DATE_FMT="+%Y-%m-%dT%H:%M:%SZ"
if [ -z "${SOURCE_DATE_EPOCH-}" ]; then
    BUILD_DATE=$(date -u ${DATE_FMT})
else
    BUILD_DATE=$(date -u -d "${SOURCE_DATE_EPOCH}" "${DATE_FMT}" 2>/dev/null || date -u -r "${SOURCE_DATE_EPOCH}" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
fi

COSIGN_FLAGS="-a GIT_HASH=$(git rev-parse HEAD) -a GIT_VERSION=${RELEASE_VERSION} -a BUILD_DATE=${BUILD_DATE}"

echo "running release4"

authenticate() {
    aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin ${RELEASE_REPO}
}

authenticatePrivateRepo() {
  aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin ${PRIVATE_PULL_THROUGH_HOST}
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

notifyIfStableRelease() {
  COMMIT_TAG=$(git describe --tags --exact-match || echo "none")
  if [[ "${COMMIT_TAG}" == "none" || "${COMMIT_TAG}" != v* ]]; then
    echo "Not sending a stable release message since no valid stable tag releases found in tags of this commit: '${COMMIT_TAG}'"
    return
  fi
  notifyRelease "stable" $COMMIT_TAG
}

cosignImages() {
    COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} ${CONTROLLER_DIGEST}
    COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} ${WEBHOOK_DIGEST}
}

notifyRelease() {
    RELEASE_TYPE=$1
    RELEASE_IDENTIFIER=$2
    MESSAGE="{\"releaseType\":\"${RELEASE_TYPE}\",\"releaseIdentifier\":\"${RELEASE_IDENTIFIER}\"}"
    aws sns publish \
        --topic-arn "arn:aws:sns:us-east-1:071440425669:KarpenterReleases" \
        --message ${MESSAGE} \
        --no-cli-pager
}

pullPrivateReplica(){
  authenticatePrivateRepo
  RELEASE_TYPE=$1
  RELEASE_IDENTIFIER=$2
  PULL_THROUGH_CACHE_PATH="${PRIVATE_PULL_THROUGH_HOST}/ecr-public/karpenter/"

  docker pull "${PULL_THROUGH_CACHE_PATH}controller:${RELEASE_IDENTIFIER}"
  docker pull "${PULL_THROUGH_CACHE_PATH}webhook:${RELEASE_IDENTIFIER}"
}

publishHelmChart() {
    HELM_CHART_FILE_NAME="karpenter-${HELM_CHART_VERSION}.tgz"

    cd charts
    helm lint karpenter
    helm package karpenter --version $HELM_CHART_VERSION
    helm push "${HELM_CHART_FILE_NAME}" "oci://${RELEASE_REPO}"
    rm "${HELM_CHART_FILE_NAME}"
}
