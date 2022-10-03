#!/usr/bin/env bash
set -euo pipefail

IS_STABLE_RELEASE=false
if [ -z ${SNAPSHOT_TAG+x} ];then
  SNAPSHOT_TAG=${SNAPSHOT_TAG:-$(git rev-parse HEAD)}
else
  if [[ "${1-$SNAPSHOT_TAG}" == v* ]]; then
    IS_STABLE_RELEASE=true
  fi
fi

RELEASE_REPO=${RELEASE_REPO:-public.ecr.aws/karpenter/}

if [[ $IS_STABLE_RELEASE == true ]]; then
  HELM_CHART_VERSION=$SNAPSHOT_TAG
fi

echo "Releasing ${SNAPSHOT_TAG}. IS_STABLE_RELEASE: ${IS_STABLE_RELEASE}"

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/release_common.sh"

authenticate
buildImages $HELM_CHART_VERSION
cosignImages
publishHelmChart

if [[ $IS_STABLE_RELEASE == true ]]; then
    notifyRelease "stable" $SNAPSHOT_TAG
    website
    editWebsiteConfig
else
    pullPrivateReplica "snapshot" $SNAPSHOT_TAG
    notifyRelease "snapshot" $HELM_CHART_VERSION
fi
