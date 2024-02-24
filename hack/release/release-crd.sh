#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
# shellcheck source=hack/release/common.sh
source "${SCRIPT_DIR}/common.sh"

commit_sha="$(git rev-parse HEAD)"
git_tag="$(git describe --exact-match --tags || echo "no tag")"

BUILD_DATE="$(buildDate "$(dateEpoch)")"

publishHelmChart "${RELEASE_REPO_GH}" "karpenter-crd" "${commit_sha}" "${commit_sha}" "${BUILD_DATE}"

if [[ "${git_tag}" == v* ]]; then
  publishHelmChart "${RELEASE_REPO_GH}" "karpenter-crd" "${git_tag#v}" "${commit_sha}" "${BUILD_DATE}"
fi
