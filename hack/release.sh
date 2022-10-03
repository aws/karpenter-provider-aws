#!/usr/bin/env bash
set -euo pipefail

IS_STABLE_RELEASE=false
if [ -z ${SNAPSHOT_TAG+x} ];then
 echo "SNAPSHOT_TAG is not set"
else
  if [[ "${1-$SNAPSHOT_TAG}" == v* ]]; then
    IS_STABLE_RELEASE=true
  fi
fi

SNAPSHOT_TAG=${SNAPSHOT_TAG:-$(git rev-parse HEAD)}
RELEASE_REPO=${RELEASE_REPO:-public.ecr.aws/karpenter/}

if [[ $IS_STABLE_RELEASE ]]; then
  HELM_CHART_VERSION=$SNAPSHOT_TAG
fi

echo "Releasing ${SNAPSHOT_TAG}. IS_STABLE_RELEASE: ${IS_STABLE_RELEASE}"

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/release_common.sh"

authenticate
buildImages $HELM_CHART_VERSION
cosignImages
publishHelmChart

if [[ $IS_STABLE_RELEASE ]]; then
    notifyRelease "stable" $SNAPSHOT_TAG
    website
    editWebsiteConfig
else
    pullPrivateReplica "snapshot" $SNAPSHOT_TAG
    notifyRelease "snapshot" $HELM_CHART_VERSION
fi
