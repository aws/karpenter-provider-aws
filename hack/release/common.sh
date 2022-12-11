#!/usr/bin/env bash
set -euo pipefail

config(){
  GITHUB_ACCOUNT="aws"
  AWS_ACCOUNT_ID="071440425669"
  ECR_GALLERY_NAME="karpenter"
  RELEASE_REPO=${RELEASE_REPO:-public.ecr.aws/${ECR_GALLERY_NAME}/}
  RELEASE_REPO_GH=${RELEASE_REPO_GH:-ghcr.io/${GITHUB_ACCOUNT}/karpenter}

  PRIVATE_PULL_THROUGH_HOST="${AWS_ACCOUNT_ID}.dkr.ecr.us-east-1.amazonaws.com"
  SNS_TOPIC_ARN="arn:aws:sns:us-east-1:${AWS_ACCOUNT_ID}:KarpenterReleases"
  CURRENT_MAJOR_VERSION="0"
  RELEASE_PLATFORM="--platform=linux/amd64,linux/arm64"
}

setEnvVariables(){
  if [ -z ${RELEASE_VERSION+x} ];then
    echo "Required env variable RELEASE_VERSION not set"
    exit 1
  fi

  IS_STABLE_RELEASE=false
  if [[ "${RELEASE_VERSION}" == v* ]]; then
    IS_STABLE_RELEASE=true
  fi

  if [[ $IS_STABLE_RELEASE == true ]]; then
    HELM_CHART_VERSION=$RELEASE_VERSION
  else
    HELM_CHART_VERSION="v${CURRENT_MAJOR_VERSION}-${RELEASE_VERSION}"
  fi

  echo "Is Stable Release? ${IS_STABLE_RELEASE}, Helm Chart Version ${HELM_CHART_VERSION} "

  # TODO restore https://reproducible-builds.org/docs/source-date-epoch/
  DATE_FMT="+%Y-%m-%dT%H:%M:%SZ"
  if [ -z "${SOURCE_DATE_EPOCH-}" ]; then
      BUILD_DATE=$(date -u ${DATE_FMT})
  else
      BUILD_DATE=$(date -u -d "${SOURCE_DATE_EPOCH}" "${DATE_FMT}" 2>/dev/null || date -u -r "${SOURCE_DATE_EPOCH}" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
  fi

  COSIGN_FLAGS="-a GIT_HASH=$(git rev-parse HEAD) -a GIT_VERSION=${RELEASE_VERSION} -a BUILD_DATE=${BUILD_DATE}"
}

authenticate() {
    aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin ${RELEASE_REPO}
}

authenticatePrivateRepo() {
  aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin ${PRIVATE_PULL_THROUGH_HOST}
}

buildImages() {
    CONTROLLER_DIGEST=$(GOFLAGS=${GOFLAGS} KO_DOCKER_REPO=${RELEASE_REPO} ko publish -B -t ${RELEASE_VERSION} ${RELEASE_PLATFORM} ./cmd/controller)
    yq e -i ".controller.image = \"${CONTROLLER_DIGEST}\"" charts/karpenter/values.yaml
    yq e -i ".appVersion = \"${RELEASE_VERSION#v}\"" charts/karpenter/Chart.yaml
    yq e -i ".version = \"${HELM_CHART_VERSION#v}\"" charts/karpenter/Chart.yaml
}

cosignImages() {
    COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} ${CONTROLLER_DIGEST}
}

notifyRelease() {
    RELEASE_TYPE=$1
    RELEASE_IDENTIFIER=$2
    MESSAGE="{\"releaseType\":\"${RELEASE_TYPE}\",\"releaseIdentifier\":\"${RELEASE_IDENTIFIER}\"}"
    aws sns publish \
        --topic-arn ${SNS_TOPIC_ARN} \
        --message ${MESSAGE} \
        --no-cli-pager
}

pullPrivateReplica(){
  authenticatePrivateRepo
  RELEASE_TYPE=$1
  RELEASE_IDENTIFIER=$2
  PULL_THROUGH_CACHE_PATH="${PRIVATE_PULL_THROUGH_HOST}/ecr-public/${ECR_GALLERY_NAME}/"

  docker pull "${PULL_THROUGH_CACHE_PATH}controller:${RELEASE_IDENTIFIER}"
}

publishHelmChart() {
    HELM_CHART_FILE_NAME="karpenter-${HELM_CHART_VERSION}.tgz"

    cd charts
    helm lint karpenter
    helm package karpenter --version $HELM_CHART_VERSION
    helm push "${HELM_CHART_FILE_NAME}" "oci://${RELEASE_REPO}"
    rm "${HELM_CHART_FILE_NAME}"
    cd ..
}

publishHelmChartToGHCR() {
    CHART_NAME=$1
    HELM_CHART_VERSION=$2
    HELM_CHART_FILE_NAME="${CHART_NAME}-${HELM_CHART_VERSION}.tgz"

    cd charts
    helm dependency update "${CHART_NAME}"
    helm lint "${CHART_NAME}"
    helm package "${CHART_NAME}" --version $HELM_CHART_VERSION
    helm push "${HELM_CHART_FILE_NAME}" "oci://${RELEASE_REPO_GH}"
    rm "${HELM_CHART_FILE_NAME}"
    cd ..
}

createNewWebsiteDirectory() {
    RELEASE_VERSION=$1
    mkdir -p website/content/en/${RELEASE_VERSION}
    cp -r website/content/en/preview/* website/content/en/${RELEASE_VERSION}/
    find website/content/en/${RELEASE_VERSION}/ -type f | xargs perl -i -p -e "s/{{< param \"latest_release_version\" >}}/${RELEASE_VERSION}/g;"
    find website/content/en/${RELEASE_VERSION}/*/*/*.yaml -type f | xargs perl -i -p -e "s/preview/${RELEASE_VERSION}/g;"
}

editWebsiteConfig() {
  RELEASE_VERSION=$1

  # sed has a different syntax on mac
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' '/^\/docs\/\*/d' website/static/_redirects
  else
    sed -i '/^\/docs\/\*/d' website/static/_redirects
  fi

  echo "/docs/*     	                /${RELEASE_VERSION}/:splat" >>website/static/_redirects

  yq -i ".params.latest_release_version = \"${RELEASE_VERSION}\"" website/config.yaml
  yq -i ".menu.main[] |=select(.name == \"Docs\") .url = \"${RELEASE_VERSION}\"" website/config.yaml
}

# editWebsiteVersionsMenu sets relevant releases in the version dropdown menu of the website
# without increasing the size of the set.
# TODO: We need to maintain a list of latest minors here only. This is not currently
# easy to achieve, however when we have a major release and we decide to maintain
# a selected minor releases we can maintain that list in the repo and use it in here
editWebsiteVersionsMenu() {
  RELEASE_VERSION=$1
  VERSIONS=(${RELEASE_VERSION})
  while IFS= read -r LINE; do
    SANITIZED_VERSION=$(echo "${LINE}" | sed -e 's/["-]//g' -e 's/ *//g')
    VERSIONS+=("${SANITIZED_VERSION}")
  done < <(yq '.params.versions' website/config.yaml)
  unset VERSIONS[${#VERSIONS[@]}-2]

  yq -i '.params.versions = []' website/config.yaml

  for VERSION in "${VERSIONS[@]}"; do
    yq -i ".params.versions += \"${VERSION}\"" website/config.yaml
  done
}
