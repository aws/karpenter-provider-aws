#!/usr/bin/env bash
set -euo pipefail

# check-lts.sh stops a new minor release before anything is published when the LTS list on the default branch is
# overdue for a new entry. See requireRecentLTS in common.sh. Patch releases and pre-releases are not checked.

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
# shellcheck source=hack/release/common.sh
source "${SCRIPT_DIR}/common.sh"

git_tag="${RELEASE_TAG:-$(git describe --exact-match --tags 2>/dev/null || true)}"
if [[ ! "${git_tag}" =~ ^v[0-9]+\.[0-9]+\.0$ ]]; then
  echo "${git_tag:-HEAD} is not a new minor release, skipping the LTS check"
  exit 0
fi
# The release is tagged on the default branch, but read the list from its tip so that a fix merged after tagging
# takes effect when the release is re-run
requireRecentLTS "$(git show "${LTS_CONFIG_REF:-origin/main}:website/hugo.yaml")"
echo "The LTS list is up to date"
