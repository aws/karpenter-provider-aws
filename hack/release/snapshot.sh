#!/usr/bin/env bash
set -euo pipefail

GIT_TAG=$(git describe --exact-match --tags || echo "no tag")
HEAD_HASH=$(git rev-parse HEAD)

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/common.sh"
config

snapshot "$HEAD_HASH"
