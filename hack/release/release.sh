#!/usr/bin/env bash
set -euo pipefail

GIT_TAG=$(git describe --exact-match --tags || echo "no tag")
if [[ "$GIT_TAG" == "no tag" ]]; then
    echo "Failed to release: commit is untagged"
    exit 1
fi
HEAD_HASH=$(git rev-parse HEAD)

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/common.sh"
config

# Don't release with a dirty commit!
if [[ "$(git status --porcelain)" != "" ]]; then
    exit 1
fi

release "$GIT_TAG"
