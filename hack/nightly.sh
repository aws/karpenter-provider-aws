#!/bin/bash -e

git tag $(date "+nightly.%m.%d.%y")

RELEASE_REPO=${RELEASE_REPO:-public.ecr.aws/karpenter}
RELEASE_VERSION=${RELEASE_VERSION:-$(git describe --tags --always)}
RELEASE_PLATFORM="--platform=linux/amd64,linux/arm64"

# TODO restore https://reproducible-builds.org/docs/source-date-epoch/
DATE_FMT="+%Y-%m-%dT%H:%M:%SZ"
if [ -z "$SOURCE_DATE_EPOCH" ]; then
    BUILD_DATE=$(date -u ${DATE_FMT})
else
    BUILD_DATE=$(date -u -d "${SOURCE_DATE_EPOCH}" "${DATE_FMT}" 2>/dev/null || date -u -r "${SOURCE_DATE_EPOCH}" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
fi
COSIGN_FLAGS="-a GIT_HASH=$(git rev-parse HEAD) -a GIT_VERSION=${RELEASE_VERSION} -a BUILD_DATE=${BUILD_DATE}"

echo "
KO_DOCKER_REPO $KO_DOCKER_REPO
RELEASE_REPO $RELEASE_REPO
RELEASE_VERSION: $RELEASE_VERSION
RELEASE_PLATFORM $RELEASE_PLATFORM
DATE_FMT $DATE_FMT
BUILD_DATE $BUILD_DATE
COSIGN_FLAGS $COSIGN_FLAGS"

image() {
    aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin ${RELEASE_REPO}
    CONTROLLER_DIGEST=$(GOFLAGS=${GOFLAGS} KO_DOCKER_REPO=${RELEASE_REPO} ko publish -B -t ${RELEASE_VERSION} ${RELEASE_PLATFORM} ./cmd/controller)
    WEBHOOK_DIGEST=$(GOFLAGS=${GOFLAGS} KO_DOCKER_REPO=${RELEASE_REPO} ko publish -B -t ${RELEASE_VERSION} ${RELEASE_PLATFORM} ./cmd/webhook)
    yq e -i ".controller.image = \"${CONTROLLER_DIGEST}\"" charts/karpenter/values.yaml
    yq e -i ".webhook.image = \"${WEBHOOK_DIGEST}\"" charts/karpenter/values.yaml
    yq e -i ".appVersion = \"${RELEASE_VERSION#v}\"" charts/karpenter/Chart.yaml
    yq e -i ".version = \"${RELEASE_VERSION#v}\"" charts/karpenter/Chart.yaml
    COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} ${CONTROLLER_DIGEST}
    COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} ${WEBHOOK_DIGEST}
}

image
