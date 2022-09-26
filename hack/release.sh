#!/usr/bin/env bash
set -euo pipefail

echo "running release2"

SNAPSHOT_TAG=$(git describe --tags --exact-match)
echo "running release3"
RELEASE_REPO=${RELEASE_REPO:-public.ecr.aws/d1w0j9s0}
echo "running release4"

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
echo "running release5"
source "${SCRIPT_DIR}/release_common.sh"
echo "running release6"
env

website() {
    mkdir -p website/content/en/${RELEASE_VERSION} && cp -r website/content/en/preview/* website/content/en/${RELEASE_VERSION}/
    find website/content/en/${RELEASE_VERSION}/ -type f | xargs perl -i -p -e "s/{{< param \"latest_release_version\" >}}/${RELEASE_VERSION}/g;"
    find website/content/en/${RELEASE_VERSION}/*/*/*.yaml -type f | xargs perl -i -p -e "s/preview/${RELEASE_VERSION}/g;"
}

editWebsiteConfig() {
  sed -i '' '/^\/docs\/\*/d' website/static/_redirects
  echo "/docs/*     	                /${RELEASE_VERSION}/:splat" >>website/static/_redirects

  yq -i ".params.latest_release_version = \"${RELEASE_VERSION}\"" website/config.yaml
  yq -i ".menu.main[] |=select(.name == \"Docs\") .url = \"${RELEASE_VERSION}\"" website/config.yaml

  editWebsiteVersionsMenu
}

# editWebsiteVersionsMenu sets relevant releases in the version dropdown menu of the website
# without increasing the size of the set.
# TODO: We need to maintain a list of latest minors here only. This is not currently
# easy to achieve, however when we have a major release and we decide to maintain
# a selected minor releases we can maintain that list in the repo and use it in here
editWebsiteVersionsMenu() {
  VERSIONS=(${RELEASE_VERSION})
  while IFS= read -r LINE; do
    SANITIZED_VERSION=$(echo "${LINE}" | sed -e 's/["-]//g' -e 's/ *//g')
    VERSIONS+=("${SANITIZED_VERSION}")
  done < <(yq '.params.versions' website/config.yaml)
  unset VERSIONS[${#VERSIONS[@]}-2]

  yq -i '.params.versions = []' website/config.yaml

  for VERSION in "${VERSIONS[@]}"; do
    yq -i ".params.versions += \"${VERSION}\"" website/config.yaml
  done
}

authenticate
echo "Building images for version ${RELEASE_VERSION}"
buildImages $RELEASE_VERSION
cosignImages
publishHelmChart
website
editWebsiteConfig
