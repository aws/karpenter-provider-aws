#!/usr/bin/env bash
set -euo pipefail

SNAPSHOT_TAG=$(git rev-parse HEAD)
RELEASE_REPO=${RELEASE_REPO:-public.ecr.aws/karpenter/}

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/release_common.sh"

authenticate
buildImages $HELM_CHART_VERSION
cosignImages
publishHelmChart
notifyIfStableRelease
pullPrivateReplica "snapshot" $SNAPSHOT_TAG
notifyRelease "snapshot" $HELM_CHART_VERSION
