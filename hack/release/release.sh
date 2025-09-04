#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
# shellcheck source=hack/release/common.sh
source "${SCRIPT_DIR}/common.sh"

git_tag="$(git describe --exact-match --tags || echo "no tag")"
if [[ "${git_tag}" == "no tag" ]]; then
  echo "Failed to release: commit is untagged"
  exit 1
fi
commit_sha="$(git rev-parse HEAD)"

# Don't release with a dirty commit!
if [[ "$(git status --porcelain)" != "" ]]; then
  exit 1
fi

release "${commit_sha}" "${git_tag#v}"
