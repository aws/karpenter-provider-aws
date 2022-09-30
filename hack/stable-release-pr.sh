#!/usr/bin/env bash
set -euo pipefail

STABLE_RELEASE_VERSION=$(git describe --tags --always)

if [[ -z "${1-SNAPSHOT_TAG}" ]]; then
  echo "missing required environment variable SNAPSHOT_TAG"
  exit 1
fi

git checkout -b "${SNAPSHOT_TAG}"
git add website
git commit -m "Stable Release updates Release ${SNAPSHOT_TAG}"
git push --set-upstream origin "${SNAPSHOT_TAG}"
echo "STABLE_RELEASE_VERSION=${SNAPSHOT_TAG}" >> $GITHUB_ENV
