#!/usr/bin/env bash
set -euo pipefail

 if [ -z ${RELEASE_VERSION+x} ];then
    echo "missing required environment variable RELEASE_VERSION"
    exit 1
  fi

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/common.sh"

echo "RenderingPrep website files for ${RELEASE_VERSION}"

createNewWebsiteDirectory $RELEASE_VERSION
editWebsiteConfig $RELEASE_VERSION
editWebsiteVersionsMenu $RELEASE_VERSION
