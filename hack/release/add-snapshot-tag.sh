#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 3 ]
then
  echo "Missing three required arguments. Usage: retag-snapshot-release.sh snapshot-tag new-tag-to-be-added release-type"
  exit 1
fi
SNAPSHOT_TAG=$1
NEW_TAG=$2
RELEASE_TYPE=$3

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
source "${SCRIPT_DIR}/common.sh"

tagAllRepositories(){
    tagRelease controller "${SNAPSHOT_TAG}"
    tagRelease karpenter "v0-${SNAPSHOT_TAG}"
}

tagRelease() {
   REPOSITORY=$1
   EXISTING_TAG=$2
   MANIFEST=$(docker manifest inspect "${RELEASE_REPO}${REPOSITORY}:${EXISTING_TAG}")
   aws ecr-public put-image --repository-name "${REPOSITORY}" --image-tag "${NEW_TAG}" --image-manifest "$MANIFEST" --no-cli-pager
}

config
authenticate
tagAllRepositories
pullPrivateReplica $RELEASE_TYPE $NEW_TAG
notifyRelease $RELEASE_TYPE $NEW_TAG
