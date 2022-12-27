#!/usr/bin/env bash
set -euo pipefail

#!/usr/bin/env bash
set -euo pipefail

GIT_TAG=$(git describe --exact-match --tags || echo "none")
if [ -z ${RELEASE_VERSION+x} ];then
  echo "Missing required git tag"
  exit 1
fi

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/common.sh"

echo "RenderingPrep website files for ${GIT_TAG}"

createNewWebsiteDirectory $GIT_TAG
editWebsiteConfig $GIT_TAG
editWebsiteVersionsMenu $GIT_TAG
