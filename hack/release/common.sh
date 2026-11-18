#!/usr/bin/env bash
set -euo pipefail

ECR_GALLERY_NAME="karpenter"
RELEASE_REPO_ECR="public.ecr.aws/${ECR_GALLERY_NAME}/"

SNAPSHOT_ECR="021119463062.dkr.ecr.us-east-1.amazonaws.com"
SNAPSHOT_REPO_ECR="${SNAPSHOT_ECR}/karpenter/snapshot/"

CACHED_REPO_ECR="${RELEASE_ACCOUNT_ID:-}.dkr.ecr.us-east-1.amazonaws.com"
CACHED_REPO_NAME="${CACHED_ECR_NAME:-}"

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

  authenticateSnapshotRepo
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

  authenticateCachedRepo
  pullImages "${version}"
}

authenticate() {
  aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin "${RELEASE_REPO_ECR}"
}

authenticateCachedRepo() {
  aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin "${CACHED_REPO_ECR}"
}

authenticateSnapshotRepo() {
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
  local oci_repo helm_chart version commit_sha build_date ah_config_file_name helm_chart_artifact helm_chart_digest

  oci_repo="${1}"
  helm_chart="${2}"
  version="${3}"
  commit_sha="${4}"
  build_date="${5}"

  ah_config_file_name="${helm_chart}/artifacthub-repo.yaml"
  helm_chart_artifact="${helm_chart}-${version}.tgz"

  yq e -i ".appVersion = \"${version}\"" "charts/${helm_chart}/Chart.yaml"
  yq e -i ".version = \"${version}\"" "charts/${helm_chart}/Chart.yaml"

  cd charts
  if [[ -s "${ah_config_file_name}" ]] && [[ "$oci_repo" == "${RELEASE_REPO_ECR}" ]]; then
    # ECR requires us to create an empty config file for an alternative
    # media type artifact push rather than /dev/null
    # https://github.com/aws/containers-roadmap/issues/1074
    temp=$(mktemp)
    echo {} > "${temp}"
    oras push "${oci_repo}${helm_chart}:artifacthub.io" --config "${temp}:application/vnd.cncf.artifacthub.config.v1+yaml" "${ah_config_file_name}:application/vnd.cncf.artifacthub.repository-metadata.layer.v1.yaml"
  fi
  helm dependency update "${helm_chart}"
  helm lint "${helm_chart}"
  helm package "${helm_chart}" --version "${version}"
  helm push "${helm_chart_artifact}" "oci://${oci_repo}"
  rm "${helm_chart_artifact}"
  cd ..

  helm_chart_digest="$(crane digest "${oci_repo}/${helm_chart}:${version}")"
  cosignOciArtifact "${version}" "${commit_sha}" "${build_date}" "${oci_repo}${helm_chart}:${version}@${helm_chart_digest}"
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

pullImages() {
  local tag="${1}"

  docker pull "${CACHED_REPO_ECR}"/"${CACHED_REPO_NAME}"/controller:"${tag}"
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

# websiteVersionDirectories prints the names of the docs version directories (e.g. v1.14), sorted by version
websiteVersionDirectories() {
  find website/content/en/* -maxdepth 0 -type d -name "v*" -print0 | xargs -0 -r -n 1 basename | sort -V
}

# removeOldWebsiteDirectories removes the docs version directories that are no longer kept, and redirects their pages
# to the same pages in the latest docs so that links to them keep working. The rule uses the Amplify <*> wildcard,
# which hack/docs/parse-redirects passes through unexpanded. It keeps the last n minors, the supported
# LTS minors in params.lts_versions, and the preview, docs, and v1.0 directories.
removeOldWebsiteDirectories() {
  local n=3 keep dir version

  # Keep the v1.0 version around while we are supporting v1beta1 migration
  # Drop it once we no longer want to maintain the v1.0 version in the docs
  keep="$(websiteVersionDirectories | grep -v -x 'v1\.0' | tail -n "${n}")"
  keep+=$'\n'"$(yq '.params.lts_versions[].version' website/hugo.yaml)"
  keep+=$'\n'"v1.0"$'\n'"preview"$'\n'"docs"

  for dir in website/content/en/*/; do
    version="$(basename "${dir}")"
    if grep -q -x -F -- "${version}" <<< "${keep}"; then
      continue
    fi
    rm -r "${dir}"
    if ! grep -q -F "/${version}/<*>  /docs/<*>" website/static/_redirects; then
      printf '/%s/<*>  /docs/<*>\n' "${version}" >> website/static/_redirects
    fi
  done
}

editWebsiteConfig() {
  local version="${1}"

  yq -i ".params.latest_release_version = \"${version}\"" website/hugo.yaml
}

# requireLTSWebsiteDirectories fails when a supported LTS minor in params.lts_versions has no docs version directory.
# Pruning never removes them, so a missing directory means the docs were lost and have to be restored, rather than
# the minor being left out of the version menu.
requireLTSWebsiteDirectories() {
  local version missing=()

  while read -r version; do
    if [[ -n "${version}" ]] && [[ ! -d "website/content/en/${version}" ]]; then
      missing+=("${version}")
    fi
  done < <(yq '.params.lts_versions[].version' website/hugo.yaml)
  if ((${#missing[@]} > 0)); then
    echo "Supported LTS minors in params.lts_versions have no docs: ${missing[*]}. Restore website/content/en/<version>." >&2
    return 1
  fi
}

# editWebsiteVersionsMenu sets the version dropdown menu of the website to the supported docs versions: the latest
# minor, the supported LTS minors in params.lts_versions, v1.0 (kept for v1beta1 migration), and preview.
# Other version directories that are still kept are left out of the menu. Their pages show a banner that they are no
# longer maintained (website/layouts/partials/version-banner.html).
editWebsiteVersionsMenu() {
  local latest versions version

  requireLTSWebsiteDirectories
  latest="$(websiteVersionDirectories | tail -n 1)"
  versions=("${latest}")
  while read -r version; do
    if [[ -n "${version}" ]] && [[ "${version}" != "${latest}" ]]; then
      versions+=("${version}")
    fi
  done < <(yq '.params.lts_versions[].version' website/hugo.yaml | sort -r -V)
  if [[ "${latest}" != "v1.0" ]] && [[ -d website/content/en/v1.0 ]]; then
    versions+=('v1.0')
  fi
  versions+=('preview')

  yq -i '.params.versions = []' website/hugo.yaml

  for version in "${versions[@]}"; do
    yq -i ".params.versions += \"${version}\"" website/hugo.yaml
  done
}

# requireRecentLTS fails when the newest LTS minor in a hugo.yaml (passed as its contents) was released more than 6
# months ago. The support policy designates a new LTS minor every 6 months, so an older list means a designation was
# missed, and pruning would delete the docs of the unlisted LTS minor like any other minor's. LTS_CHECK_NOW (YYYY-MM)
# overrides the current month, for testing.
requireRecentLTS() {
  local hugo_config now newest=0 version released months entries

  hugo_config="${1}"
  now="${LTS_CHECK_NOW:-$(date -u +%Y-%m)}"
  # Read the list before the loop, so that a parse failure fails the check instead of being hidden by the loop
  entries="$(yq '.params.lts_versions[] | [.version, .released] | @tsv' <<< "${hugo_config}")"
  while IFS=$'\t' read -r version released; do
    if [[ -z "${version}" ]]; then
      continue
    fi
    months=$(($(date -u -d "1 ${released}" +%Y) * 12 + 10#$(date -u -d "1 ${released}" +%m)))
    if ((months > newest)); then
      newest="${months}"
    fi
  done <<< "${entries}"
  months=$((10#${now%-*} * 12 + 10#${now#*-}))
  if ((months - newest > 6)); then
    echo "No LTS minor has been designated in the last 6 months: the newest in params.lts_versions in website/hugo.yaml on main was released more than 6 months ago." >&2
    echo "Add the new LTS minor there, with its release and end-of-life months, then re-run this release." >&2
    return 1
  fi
}

# isPatchRelease returns success when the version (e.g. 1.14.2) is a stable patch release rather than a new minor
isPatchRelease() {
  local version="${1}"

  [[ "${version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] && [[ "${version##*.}" != "0" ]]
}

# isLatestMinor returns success when the version belongs to the minor of the highest stable release tag
isLatestMinor() {
  local version="${1}" latest

  latest="$(git tag -l 'v*' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n 1)"
  latest="${latest#v}"
  [[ "${version%.*}" == "${latest%.*}" ]]
}

# bumpPatchVersion replaces the previous patch version of a minor with the new patch version in a website content directory.
# The previous version is read from KARPENTER_VERSION in the getting started guide. The upgrade guide and compatibility
# matrix reference historical versions and are skipped, as are feature state annotations.
bumpPatchVersion() {
  local version dir previous previous_regex

  version="${1}"
  dir="${2}"

  previous="$(sed -n -E 's/^export KARPENTER_VERSION="([0-9.]+)"$/\1/p' "${dir}/getting-started/getting-started-with-karpenter/_index.md")"
  if [[ -z "${previous}" ]] || [[ "${previous}" == "${version}" ]]; then
    return
  fi
  echo "Updating ${dir} from ${previous} to ${version}"

  previous_regex="${previous//./\\.}"
  # shellcheck disable=SC2038
  find "${dir}" -type f ! -name "upgrade-guide.md" ! -name "compatibility.md" | xargs perl -i -p -e "s/(?<![0-9.])${previous_regex}(?![0-9])/${version}/g unless /Feature State/;"
}
