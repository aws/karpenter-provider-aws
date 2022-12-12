#!/usr/bin/env bash
set -euo pipefail

HEAD_HASH=$(git rev-parse HEAD)
if [ -z ${RELEASE_VERSION+x} ];then
  RELEASE_VERSION=${HEAD_HASH}
fi

echo "Releasing ${RELEASE_VERSION}, Commit: ${HEAD_HASH}"

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/common.sh"

config
setEnvVariables
authenticate
buildImages
cosignImages
# TODO: re-enable after fixing the authentication problem
#publishHelmChartToGHCR "karpenter-crd" "${HELM_CHART_VERSION}"
publishHelmChart

if [[ $IS_STABLE_RELEASE == true ]]; then
    notifyRelease "stable" $RELEASE_VERSION
else
    pullPrivateReplica "snapshot" $RELEASE_VERSION
    notifyRelease "snapshot" $HELM_CHART_VERSION
fi
