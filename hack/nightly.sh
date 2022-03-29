#!/bin/bash -e
NIGHTLY_TAG_FMT="+%Y%m%d"
RELEASE_REPO=${RELEASE_REPO:-public.ecr.aws/z3c6l6z9/karpenter-nightly}
RELEASE_VERSION=${RELEASE_VERSION:-$(date "${NIGHTLY_TAG_FMT}")}
RELEASE_PLATFORM="--platform=linux/amd64,linux/arm64"

if [ -z "$CLOUD_PROVIDER" ]; then
    echo "CLOUD_PROVIDER environment variable is not set: 'export CLOUD_PROVIDER=aws'"
    exit 1
fi

image() {
    aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin ${RELEASE_REPO}
    CONTROLLER_DIGEST=$(GOFLAGS=${GOFLAGS} KO_DOCKER_REPO=${RELEASE_REPO} ko publish -B -t ${RELEASE_VERSION} ${RELEASE_PLATFORM} ./cmd/controller)
    WEBHOOK_DIGEST=$(GOFLAGS=${GOFLAGS} KO_DOCKER_REPO=${RELEASE_REPO} ko publish -B -t ${RELEASE_VERSION} ${RELEASE_PLATFORM} ./cmd/webhook)
}

image
