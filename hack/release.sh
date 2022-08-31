#!/usr/bin/env bash
set -euo pipefail

SNAPSHOT_TAG=$(git describe --tags --always)
RELEASE_REPO=${RELEASE_REPO:-public.ecr.aws/karpenter}

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/release_common.sh"

chart() {
    (
        cd charts
        helm lint karpenter
        helm package karpenter
        helm repo index .
        helm-docs
    )
}

website() {
    mkdir -p website/content/en/${RELEASE_VERSION} && cp -r website/content/en/preview/* website/content/en/${RELEASE_VERSION}/
    find website/content/en/${RELEASE_VERSION}/ -type f | xargs perl -i -p -e "s/{{< param \"latest_release_version\" >}}/${RELEASE_VERSION}/g;"
    find website/content/en/${RELEASE_VERSION}/*/*/*.yaml -type f | xargs perl -i -p -e "s/preview/${RELEASE_VERSION}/g;"
}

editWebsiteConfig() {
    yq -i ".params.latest_release_version = \"${RELEASE_VERSION}\"" website/config.yaml
    yq -i ".menu.main[] |=select(.name == \"Docs\") .url = \"${RELEASE_VERSION}\"" website/config.yaml

    sed -i '' '/^\/docs\/\*/d' website/static/_redirects
    echo "/docs/*     	                /${RELEASE_VERSION}/:splat" >> website/static/_redirects
}

authenticate
buildImages $RELEASE_VERSION
cosignImages
chart
website
editWebsiteConfig
