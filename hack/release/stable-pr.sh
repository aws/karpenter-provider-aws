#!/usr/bin/env bash
set -euo pipefail

STABLE_RELEASE_VERSION=$(git describe --tags --always)

if [[ -z "${1-RELEASE_VERSION}" ]]; then
  echo "missing required environment variable RELEASE_VERSION"
  exit 1
fi

git config user.name "StableRelease"
git config user.email "StableRelease@users.noreply.github.com"
git remote set-url origin https://x-access-token:${GITHUB_TOKEN}@github.com/${GITHUB_REPO}
git config pull.rebase false

BRANCH_NAME="release-${RELEASE_VERSION}"
git checkout -b "${BRANCH_NAME}"
git add website
git add charts
git commit -m "Stable Release updates Release ${RELEASE_VERSION}."
git push --set-upstream origin "${BRANCH_NAME}"
echo "STABLE_RELEASE_VERSION=${RELEASE_VERSION}" >> $GITHUB_ENV
