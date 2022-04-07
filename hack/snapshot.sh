#!/bin/bash -e
SNAPSHOT_TAG=$(git rev-parse HEAD)
RELEASE_REPO=${RELEASE_REPO:-public.ecr.aws/z4v8y7u8/}
RELEASE_VERSION=${RELEASE_VERSION:-"${SNAPSHOT_TAG}"}
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

chart() {
    (
        cd charts
        helm lint karpenter
        helm package karpenter
        helm repo index .
        helm-docs
    )
}

chart
