#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/common.sh"

config

GIT_TAG=$(git describe --exact-match --tags || echo "none")
if [[ $(releaseType $GIT_TAG) != $RELEASE_TYPE_STABLE ]]; then
  echo "Not a stable release. Missing required git tag."
  exit 1
fi

updateKarpenterCoreGoMod $GIT_TAG

git config user.name "StableRelease"
git config user.email "StableRelease@users.noreply.github.com"
git remote set-url origin https://x-access-token:${GITHUB_TOKEN}@github.com/${GITHUB_REPO}
git config pull.rebase false

BRANCH_NAME="release-${GIT_TAG}"
git checkout -b "${BRANCH_NAME}"
git add go.mod
git add go.sum
git add test/go.mod
git add test/go.sum
git add website
git add charts/karpenter/Chart.yaml
git add charts/karpenter/Chart.lock
git add charts/karpenter/values.yaml
git add charts/karpenter/README.md
git commit -m "Stable Release updates Release ${GIT_TAG}."
git push --set-upstream origin "${BRANCH_NAME}"
