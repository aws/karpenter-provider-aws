#!/bin/bash -e

RELEASE_REPO=${RELEASE_REPO:-public.ecr.aws/karpenter}
RELEASE_VERSION=${RELEASE_VERSION:-$(git describe --tags --always)}
RELEASE_PLATFORM="--platform=linux/amd64,linux/arm64"

if [ -z "$CLOUD_PROVIDER" ]; then
    echo "CLOUD_PROVIDER environment variable is not set: 'export CLOUD_PROVIDER=aws'"
    exit 1
fi

# TODO restore https://reproducible-builds.org/docs/source-date-epoch/
DATE_FMT="+%Y-%m-%dT%H:%M:%SZ"
if [ -z "$SOURCE_DATE_EPOCH" ]; then
    BUILD_DATE=$(date -u ${DATE_FMT})
else
    BUILD_DATE=$(date -u -d "${SOURCE_DATE_EPOCH}" "${DATE_FMT}" 2>/dev/null || date -u -r "${SOURCE_DATE_EPOCH}" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
fi
COSIGN_FLAGS="-a GIT_HASH=$(git rev-parse HEAD) -a GIT_VERSION=${RELEASE_VERSION} -a BUILD_DATE=${BUILD_DATE}"

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

chart() {
    (
        cd charts
        helm lint karpenter
        helm package karpenter
        helm repo index .
        helm-docs
    )
}

website() {
    mkdir -p website/content/en/${RELEASE_VERSION} && cp -r website/content/en/preview/* website/content/en/${RELEASE_VERSION}/
    find website/content/en/${RELEASE_VERSION}/ -type f | xargs perl -i -p -e "s/{{< param \"latest_release_version\" >}}/${RELEASE_VERSION}/g;"
    # make only the latest version searchable
    find website/content/en/ -type f | xargs perl -i -p -e "s/exclude_search:.*/exclude_search: true/"
    find website/content/en/${RELEASE_VERSION}/ -type f | xargs perl -i -p -e "s/exclude_search:.*/exclude_search: false/"
}

image
chart
website
