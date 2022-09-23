#!/usr/bin/env bash
set -euo pipefail

STABLE_RELEASE_VERSION=$(git describe --tags --always)

gitOpenAndPullBranch() {
  git fetch origin
  git checkout -b checkout "${STABLE_RELEASE_VERSION}"
}

gitCommitAndPush() {
  git commit -m "Stable Release updates Release ${STABLE_RELEASE_VERSION}"
  git push --set-upstream origin "${STABLE_RELEASE_VERSION}"
}

gitOpenAndPullBranch
gitCommitAndPush
