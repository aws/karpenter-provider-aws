#!/usr/bin/env bash

GH_REPO="${GH_REPO:-aws/karpenter-provider-aws}"
RELEASE_COUNT="${RELEASE_COUNT:-10}"

# Translates a git tag to the corresponding ECR tag. Starting with Karpenter v0.35.0 the leading v is dropped in the ECR tag.
function getECRTag() {
  tag="${1}"
  major=$(echo "${tag}" | sed -E 's/v([[:digit:]]+).*/\1/')
  minor=$(echo "${tag}" | sed -E 's/v[[:digit:]]+\.([[:digit:]]+).*/\1/')
  if [ "${major}" -gt "0" ] || [ "${minor}" -gt "34" ]; then
    tag=$(echo "${tag}" | tr -d 'v')
  fi
  echo "${tag}"
}

# Pull the OCI artifacts / controller images for the latest releases
while IFS= read -r tag; do
  tag="$(getECRTag "${tag}")"
  for artifact in "karpenter" "karpenter-crd"; do
    if ! helm pull "oci://public.ecr.aws/karpenter/${artifact}" --version "${tag}"; then
      printf "failed to pull OCI artifact from ECR (artifact: %s, tag: %s)\n" "${image}" "${tag}"
      exit 1
    fi
  done
  if ! docker pull "public.ecr.aws/karpenter/controller:${tag}"; then
    printf "failed to pull controller image from ECR (tag: %s)\n" "${tag}"
    exit 1
  fi
done < <(gh release list --repo "${GH_REPO}" --json tagName -L "${RELEASE_COUNT}" | go tool -modfile=go.tools.mod yq '.[].tagName')

# Check that the charts.karpenter.sh repo is still working until removed
helm repo add karpenter https://charts.karpenter.sh/
helm repo update
if ! helm pull karpenter/karpenter; then
  printf "failed to pull chart from charts.karpenter.sh"
  exit 1
fi
