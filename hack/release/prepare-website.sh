#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
# shellcheck source=hack/release/common.sh
source "${SCRIPT_DIR}/common.sh"

git_tag="${GIT_TAG:-$(git describe --exact-match --tags || echo "none")}"
if [[ "${git_tag}" != v* ]]; then
  echo "Not a stable release. Missing required git tag."
  exit 1
fi
echo "RenderingPrep website files for ${git_tag}"

prepareWebsite "${git_tag#v}"
