#!/usr/bin/env bash
set -euo pipefail

HEAD_HASH=$(git rev-parse HEAD)
GIT_TAG=$(git describe --exact-match --tags || echo "no tag")

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/common.sh"

config
publishHelmChartToGHCR "karpenter-crd" "${HEAD_HASH}"

if [[ $(releaseType $GIT_TAG) == $RELEASE_TYPE_STABLE ]]; then
  publishHelmChartToGHCR "karpenter-crd" "${GIT_TAG}"
fi
