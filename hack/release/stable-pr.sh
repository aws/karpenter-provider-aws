#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
# shellcheck source=hack/release/common.sh
source "${SCRIPT_DIR}/common.sh"

git_tag="$(git describe --exact-match --tags || echo "none")"
if [[ "${git_tag}" != v* ]]; then
  echo "Not a stable release. Missing required git tag."
  exit 1
fi

git config user.name "StableRelease"
git config user.email "StableRelease@users.noreply.github.com"
git remote set-url origin "https://x-access-token:${GITHUB_TOKEN}@github.com/${GITHUB_REPO}"
git config pull.rebase false

branch_name="release-${git_tag}"
git checkout -b "${branch_name}"
git add go.mod
git add go.sum
git add hack/docs
git add website
git add charts/karpenter-crd/Chart.yaml
git add charts/karpenter/Chart.yaml
git add charts/karpenter/Chart.lock
git add charts/karpenter/values.yaml
git add charts/karpenter/README.md
git commit -m "Stable Release updates Release ${git_tag}."
git push --set-upstream origin "${branch_name}"
