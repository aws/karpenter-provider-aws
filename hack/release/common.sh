#!/usr/bin/env bash
set -euo pipefail

ECR_GALLERY_NAME="karpenter"
RELEASE_REPO_ECR="${RELEASE_REPO_ECR:-public.ecr.aws/${ECR_GALLERY_NAME}/}"

SNAPSHOT_ECR="021119463062.dkr.ecr.us-east-1.amazonaws.com"
SNAPSHOT_REPO_ECR="${SNAPSHOT_REPO_ECR:-${SNAPSHOT_ECR}/karpenter/snapshot/}"

CURRENT_MAJOR_VERSION="0"

snapshot() {
  local commit_sha version helm_chart_version

  commit_sha="${1}"
  version="${commit_sha}"
  helm_chart_version="${CURRENT_MAJOR_VERSION}-${commit_sha}"

  echo "Release Type: snapshot
Release Version: ${version}
Commit: ${commit_sha}
Helm Chart Version ${helm_chart_version}"

  authenticatePrivateRepo
  build "${SNAPSHOT_REPO_ECR}" "${version}" "${helm_chart_version}" "${commit_sha}"
}

release() {
  local commit_sha version helm_chart_version

  commit_sha="${1}"
  version="${2}"
  helm_chart_version="${version}"

  echo "Release Type: stable
Release Version: ${version}
Commit: ${commit_sha}
Helm Chart Version ${helm_chart_version}"

  authenticate
  build "${RELEASE_REPO_ECR}" "${version}" "${helm_chart_version}" "${commit_sha}"
}

authenticate() {
  aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin "${RELEASE_REPO_ECR}"
}

authenticatePrivateRepo() {
  aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin "${SNAPSHOT_ECR}"
}

build() {
  local oci_repo version helm_chart_version commit_sha date_epoch build_date img img_repo img_tag img_digest

  oci_repo="${1}"
  version="${2}"
  helm_chart_version="${3}"
  commit_sha="${4}"

  date_epoch="$(dateEpoch)"
  build_date="$(buildDate "${date_epoch}")"

  img="$(GOFLAGS=${GOFLAGS:-} SOURCE_DATE_EPOCH="${date_epoch}" KO_DATA_DATE_EPOCH="${date_epoch}" KO_DOCKER_REPO="${oci_repo}" ko publish -B -t "${version}" ./cmd/controller)"
  img_repo="$(echo "${img}" | cut -d "@" -f 1 | cut -d ":" -f 1)"
  img_tag="$(echo "${img}" | cut -d "@" -f 1 | cut -d ":" -f 2 -s)"
  img_digest="$(echo "${img}" | cut -d "@" -f 2)"

  cosignOciArtifact "${version}" "${commit_sha}" "${build_date}" "${img}"

  yq e -i ".controller.image.repository = \"${img_repo}\"" charts/karpenter/values.yaml
  yq e -i ".controller.image.tag = \"${img_tag}\"" charts/karpenter/values.yaml
  yq e -i ".controller.image.digest = \"${img_digest}\"" charts/karpenter/values.yaml

  publishHelmChart "${oci_repo}" "karpenter" "${helm_chart_version}" "${commit_sha}" "${build_date}"
  publishHelmChart "${oci_repo}" "karpenter-crd" "${helm_chart_version}" "${commit_sha}" "${build_date}"
}

publishHelmChart() {
  local oci_repo helm_chart version commit_sha build_date helm_chart_artifact helm_chart_digest

  oci_repo="${1}"
  helm_chart="${2}"
  version="${3}"
  commit_sha="${4}"
  build_date="${5}"

  helm_chart_artifact="${helm_chart}-${version}.tgz"

  updateAhConfig "${oci_repo}" "${helm_chart}"

  yq e -i ".appVersion = \"${version}\"" "charts/${helm_chart}/Chart.yaml"
  yq e -i ".version = \"${version}\"" "charts/${helm_chart}/Chart.yaml"

  cd charts
  
  helm dependency update "${helm_chart}"
  helm lint "${helm_chart}"
  helm package "${helm_chart}" --version "${version}"
  helm push "${helm_chart_artifact}" "oci://${oci_repo}"
  rm "${helm_chart_artifact}"
  cd ..

  helm_chart_digest="$(crane digest "${oci_repo}/${helm_chart}:${version}")"
  cosignOciArtifact "${version}" "${commit_sha}" "${build_date}" "${oci_repo}${helm_chart}:${version}@${helm_chart_digest}"
}

updateAhConfig() {
  local oci_repo helm_chart ah_config_path image_config_path image_config media_type oci_repository oci_image old_config_digest blob_digest

  oci_repo="${1}"
  helm_chart="${2}"

  ah_config_path="./charts/${helm_chart}/artifacthub-repo.yaml"

  if [[ -f "${ah_config_path}" ]] && [[ "${oci_repo}" == "${RELEASE_REPO_ECR}" ]]; then
    # ECR requires us to create an empty config file for an alternative
    # media type artifact push rather than /dev/null
    # https://github.com/aws/containers-roadmap/issues/1074
    image_config_path="$(mktemp)"
    echo "{}" > "${image_config_path}"

    image_config="${image_config_path}:application/vnd.cncf.artifacthub.config.v1+yaml"
    media_type="application/vnd.cncf.artifacthub.repository-metadata.layer.v1.yaml"
    oci_repository="${oci_repo}${helm_chart}"
    oci_image="${oci_repository}:artifacthub.io"

    old_config_digest="$(crane digest "${oci_image}" || true)"

    if [[ -n "${old_config_digest}" ]]; then
      blob_digest="$(oras manifest fetch --output - "${oci_repository}@${old_config_digest}" | jq -r --arg mediaType "${media_type}" '.layers[] | select(.mediaType == $mediaType) | .digest')"

      if [[ "$(oras blob fetch --output - "${oci_repository}@${blob_digest}")" != "$(cat "${ah_config_path}")" ]]; then
        oras push --config "${image_config}" "${oci_image}" "${ah_config_path}:${media_type}"
        crane delete "${oci_repository}@${old_config_digest}"
      fi
    else
      oras push --config "${image_config}" "${oci_image}" "${ah_config_path}:${media_type}"
    fi
  fi
}

cosignOciArtifact() {
  local version commit_sha build_date artifact

  version="${1}"
  commit_sha="${2}"
  build_date="${3}"
  artifact="${4}"

  cosign sign --yes -a version="${version}" -a commitSha="${commit_sha}" -a buildDate="${build_date}" "${artifact}"
}

dateEpoch() {
  git log -1 --format='%ct'
}

buildDate() {
  local date_epoch

  date_epoch="${1}"

  date -u --date="@${date_epoch}" "+%Y-%m-%dT%H:%M:%SZ" 2>/dev/null
}

prepareWebsite() {
  local version version_parts short_version

  version="${1}"
  # shellcheck disable=SC2206
  version_parts=(${version//./ })
  short_version="${version_parts[0]}.${version_parts[1]}"

  createNewWebsiteDirectory "${short_version}" "${version}"
  removeOldWebsiteDirectories
  editWebsiteConfig "${version}"
  editWebsiteVersionsMenu
}

createNewWebsiteDirectory() {
  local short_version version

  short_version="${1}"
  version="${2}"

  mkdir -p "website/content/en/v${short_version}"
  cp -r website/content/en/preview/* "website/content/en/v${short_version}/"

  # Update parameterized variables in the preview documentation to be statically set in the versioned documentation
  # shellcheck disable=SC2038
  find "website/content/en/v${short_version}/" -type f -print | xargs perl -i -p -e "s/{{< param \"latest_release_version\" >}}/${version}/g;"
  # shellcheck disable=SC2038
  find "website/content/en/v${short_version}/" -type f | xargs perl -i -p -e "s/{{< param \"latest_k8s_version\" >}}/$(yq .params.latest_k8s_version website/hugo.yaml)/g;"
  # shellcheck disable=SC2038
  find "website/content/en/v${short_version}/"*/*/*.yaml -type f | xargs perl -i -p -e "s/preview/v${short_version}/g;"
  # shellcheck disable=SC2038
  find "website/content/en/v${short_version}/" -type f | xargs perl -i -p -e "s/{{< githubRelRef >}}/\/v${version}\//g;"

  rm -rf website/content/en/docs
  mkdir -p website/content/en/docs
  cp -r "website/content/en/v${short_version}/"* website/content/en/docs/
}

removeOldWebsiteDirectories() {
  local n=3 last_n_versions all

  # Get all the directories except the last n directories sorted from earliest to latest version
  # preview, docs, and v0.32 are special directories that we always propagate into the set of directory options
  # Keep the v0.32 version around while we are supporting v1beta1 migration
  # Drop it once we no longer want to maintain the v0.32 version in the docs
  last_n_versions=$(find website/content/en/* -maxdepth 0 -type d -name "*" | grep -v "preview\|docs\|v0.32" | sort | tail -n "${n}")
  last_n_versions+=$(echo -e "\nwebsite/content/en/preview")
  last_n_versions+=$(echo -e "\nwebsite/content/en/docs")
  last_n_versions+=$(echo -e "\nwebsite/content/en/v0.32")
  all=$(find website/content/en/* -maxdepth 0 -type d -name "*")

  ## symmetric difference
  # shellcheck disable=SC2086
  comm -3 <(sort <<< ${last_n_versions}) <(sort <<< ${all}) | tr -d '\t' | xargs -r -n 1 rm -r
}

editWebsiteConfig() {
  local version="${1}"

  yq -i ".params.latest_release_version = \"${version}\"" website/hugo.yaml
}

# editWebsiteVersionsMenu sets relevant releases in the version dropdown menu of the website
# without increasing the size of the set.
# It uses the current version directories (ignoring the docs directory) to generate this list
editWebsiteVersionsMenu() {
  local versions version

  # shellcheck disable=SC2207
  versions=($(find website/content/en/* -maxdepth 0 -type d -name "*" -print0 | xargs -0 -r -n 1 basename | grep -v "docs\|preview"))
  versions+=('preview')

  yq -i '.params.versions = []' website/hugo.yaml

  for version in "${versions[@]}"; do
    yq -i ".params.versions += \"${version}\"" website/hugo.yaml
  done
}
