#!/bin/bash -e

if [ "$#" -ne 2 ]
then
  echo "Missing two required arguments. Usage: retag-snapshot-release.sh snapshot-tag new-tag-to-be-added"
  exit 1
fi
SNAPSHOT_TAG=$1
NEW_TAG=$2
RELEASE_REPO=${RELEASE_REPO:-public.ecr.aws/z4v8y7u8/}
source release_common.sh

tagAllRepositories(){
    tagRelease controller "${SNAPSHOT_TAG}"
    tagRelease webhook "${SNAPSHOT_TAG}"
    tagRelease karpenter "${HELM_CHART_VERSION}"
}

tagRelease() {
   REPOSITORY=$1
   EXISTING_TAG=$2
   MANIFEST=$(docker manifest inspect "${PUBLIC_ECR_REGISTRY_ALIAS}${REPOSITORY}:${EXISTING_TAG}")
   aws ecr-public put-image --repository-name "${REPOSITORY}" --image-tag "${NEW_TAG}" --image-manifest "$MANIFEST"
}

authenticate
tagAllRepositories
