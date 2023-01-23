#!/usr/bin/env bash
set -euo pipefail

GIT_TAG=$(git describe --exact-match --tags || echo "no tag")
HEAD_HASH=$(git rev-parse HEAD)

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/common.sh"

config
release $HEAD_HASH #release a snapshot version

if [[ $(releaseType $GIT_TAG) == $RELEASE_TYPE_STABLE ]]; then
  release $GIT_TAG
fi
