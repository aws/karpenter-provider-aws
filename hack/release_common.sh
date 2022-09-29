#!/usr/bin/env bash
set -euo pipefail

CURRENT_MAJOR_VERSION="0"
PRIVATE_PULL_THROUGH_HOST="071440425669.dkr.ecr.us-east-1.amazonaws.com"

if [[ $SNAPSHOT_TAG != v* ]]; then
  HELM_CHART_VERSION="v${CURRENT_MAJOR_VERSION}-${SNAPSHOT_TAG}"
fi
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
    cd ..
}

website() {
    mkdir -p website/content/en/${RELEASE_VERSION}
    cp -r website/content/en/preview/* website/content/en/${RELEASE_VERSION}/
    find website/content/en/${RELEASE_VERSION}/ -type f | xargs perl -i -p -e "s/{{< param \"latest_release_version\" >}}/${RELEASE_VERSION}/g;"
    find website/content/en/${RELEASE_VERSION}/*/*/*.yaml -type f | xargs perl -i -p -e "s/preview/${RELEASE_VERSION}/g;"
}

editWebsiteConfig() {
  # sed has a different syntax on mac, to do the same on mac run:
  # sed -i '' '/^\/docs\/\*/d' website/static/_redirects
  sed -i '/^\/docs\/\*/d' website/static/_redirects
  echo "/docs/*     	                /${RELEASE_VERSION}/:splat" >>website/static/_redirects

  yq -i ".params.latest_release_version = \"${RELEASE_VERSION}\"" website/config.yaml
  yq -i ".menu.main[] |=select(.name == \"Docs\") .url = \"${RELEASE_VERSION}\"" website/config.yaml

  editWebsiteVersionsMenu
}

# editWebsiteVersionsMenu sets relevant releases in the version dropdown menu of the website
# without increasing the size of the set.
# TODO: We need to maintain a list of latest minors here only. This is not currently
# easy to achieve, however when we have a major release and we decide to maintain
# a selected minor releases we can maintain that list in the repo and use it in here
editWebsiteVersionsMenu() {
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
