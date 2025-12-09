#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
# shellcheck source=hack/release/common.sh
source "${SCRIPT_DIR}/common.sh"

commit_sha="$(git rev-parse HEAD)"

# Don't release with a dirty commit!
if [[ "$(git status --porcelain)" != "" ]]; then
  echo "Error: Git working tree is dirty. Uncommitted changes detected:"
  git status --porcelain
  exit 1
fi

snapshot "${commit_sha}"
