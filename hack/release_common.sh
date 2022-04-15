#!/bin/bash -e

PUBLIC_ECR_REGISTRY_ALIAS="public.ecr.aws/z4v8y7u8/"

SNAPSHOT_TAG=$(git rev-parse HEAD)
CURRENT_MAJOR_VERSION="0"
HELM_CHART_VERSION="v${CURRENT_MAJOR_VERSION}-${SNAPSHOT_TAG}"
RELEASE_REPO=${RELEASE_REPO:-"${PUBLIC_ECR_REGISTRY_ALIAS}"}
RELEASE_VERSION=${RELEASE_VERSION:-"${SNAPSHOT_TAG}"}
RELEASE_PLATFORM="--platform=linux/amd64,linux/arm64"

authenticate() {
  aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin ${RELEASE_REPO}
}
