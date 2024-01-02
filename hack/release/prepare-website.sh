#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/common.sh"

config

GIT_TAG=${GIT_TAG:-$(git describe --exact-match --tags || echo "none")}
if [[ $(releaseType "$GIT_TAG") != $RELEASE_TYPE_STABLE ]]; then
  echo "Not a stable release. Missing required git tag."
  exit 1
fi
echo "RenderingPrep website files for ${GIT_TAG}"

createNewWebsiteDirectory "$GIT_TAG"
removeOldWebsiteDirectories
editWebsiteConfig "$GIT_TAG"
editWebsiteVersionsMenu
