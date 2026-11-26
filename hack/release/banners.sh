#!/usr/bin/env bash
set -euo pipefail

# banners.sh keeps a managed banner block at the top of the GitHub release notes current:
# - A patch release that is older than the newest patch of its minor points to that newest patch.
# - A release of a supported Long-Term Support (LTS) minor gets the "(LTS)" title suffix and an LTS banner.
#
# Supported LTS minors and their end-of-life dates are read from params.lts_versions in website/hugo.yaml on
# LTS_CONFIG_REF (default origin/main). A minor that is removed from the list at end of life keeps its "(LTS)" title,
# but its LTS banner is no longer written.
#
# Every run reconciles every stable release, so a run that is skipped or cancelled loses nothing: the next run
# catches up. .github/workflows/release-banners.yaml runs this after every release, one run at a time.
#
# Usage: banners.sh [--dry-run]
#   --dry-run  prints the changes without editing any release

BANNER_START="<!-- karpenter-release-banner:start -->"
BANNER_END="<!-- karpenter-release-banner:end -->"

dry_run=false
for arg in "$@"; do
  case "${arg}" in
    --dry-run) dry_run=true ;;
    *)
      echo "Unknown argument: ${arg}" >&2
      exit 1
      ;;
  esac
done

: "${GITHUB_REPO:?GITHUB_REPO must be set, e.g. aws/karpenter-provider-aws}"
lts_config_ref="${LTS_CONFIG_REF:-origin/main}"

# fetchReleases prints one JSON object (tagName, name, description) per published stable vX.Y.Z release, using a
# paginated GraphQL query rather than one API call per release
fetchReleases() {
  # shellcheck disable=SC2016
  gh api graphql --paginate -F owner="${GITHUB_REPO%/*}" -F name="${GITHUB_REPO#*/}" -f query='
    query($owner: String!, $name: String!, $endCursor: String) {
      repository(owner: $owner, name: $name) {
        releases(first: 100, after: $endCursor) {
          pageInfo { hasNextPage endCursor }
          nodes { tagName name description isDraft isPrerelease }
        }
      }
    }' --jq '.data.repository.releases.nodes[] | select((.isDraft or .isPrerelease) | not) |
      select(.tagName | test("^v[0-9]+\\.[0-9]+\\.[0-9]+$")) | {tagName, name, description}'
}

# highestRelease prints the highest stable vX.Y.Z release as GitHub reports it now
highestRelease() {
  gh release list --repo "${GITHUB_REPO}" --exclude-drafts --exclude-pre-releases --limit 1000 --json tagName --jq '.[].tagName' |
    grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n 1
}

# ltsEOL prints the end-of-life date of an LTS minor (e.g. "Jul 2027" for v1.14), or nothing if the minor isn't LTS
ltsEOL() {
  local minor="${1}"

  MINOR="${minor}" yq '.params.lts_versions[] | select(.version == strenv(MINOR)) | .eol' <<< "${lts_config}"
}
# bannerBlock prints the managed banner block for a release, or nothing if the release needs no banner
bannerBlock() {
  local tag minor newest eol lines

  tag="${1}"
  minor="${2}"
  newest="${3}"
  eol="${4}"
  lines=()

  if [[ "${tag}" != "${newest}" ]]; then
    lines+=("> **Old patch release.** [${newest}](https://github.com/${GITHUB_REPO}/releases/tag/${newest}) is the latest patch for ${minor}. Please upgrade." "")
  fi
  if [[ -n "${eol}" ]]; then
    lines+=("> **This is a Long-Term Support (LTS) release.** Supported until ${eol}. See [SUPPORT.md](https://github.com/${GITHUB_REPO}/blob/main/SUPPORT.md) for details." "")
  fi
  if ((${#lines[@]} == 0)); then
    return
  fi
  printf '%s\n' "${BANNER_START}" "${lines[@]}" "${BANNER_END}"
}
# stripBanners removes the managed banner block, and with strip_legacy=1 the hand-written LTS banner that releases
# carried before this script existed. All other lines are printed unchanged, including any carriage returns.
stripBanners() {
  local strip_legacy="${1}"

  awk -v start="${BANNER_START}" -v end="${BANNER_END}" -v strip_legacy="${strip_legacy}" '
    BEGIN { top = 1 }
    {
      line = $0
      sub(/\r$/, "", line)
      if (line == start) { in_block = 1; next }
      if (in_block) { if (line == end) { in_block = 0; after_block = 1 }; next }
      if (after_block) { after_block = 0; if (line == "") next }
      if (top && strip_legacy == 1) {
        if (line ~ /^> .*\*\*This is a Long-Term Support \(LTS\) release\.\*\*$/) { legacy = 1; next }
        if (legacy && line ~ /^> Supported until /) next
        if (legacy && line == "") { legacy = 0; top = 0; next }
      }
      top = 0
      print $0
    }'
}

lts_config="$(git show "${lts_config_ref}:website/hugo.yaml")"

releases_file="$(mktemp)"
trap 'rm -f "${releases_file}"' EXIT
fetchReleases > "${releases_file}"
releases="$(jq -r .tagName "${releases_file}" | sort -V -u)"
if [[ -z "${releases}" ]]; then
  echo "No stable releases found, leaving release notes unchanged"
  exit 0
fi

changed=0
for tag in ${releases}; do
  minor="${tag%.*}"
  newest="$(grep -E "^${minor//./\\.}\.[0-9]+$" <<< "${releases}" | tail -n 1)"
  eol="$(ltsEOL "${minor}")"

  release="$(TAG="${tag}" jq -c 'select(.tagName == env.TAG)' "${releases_file}" | head -n 1)"
  name="$(jq -r '.name // ""' <<< "${release}")"
  body="$(jq -r '.description // ""' <<< "${release}")"

  new_name="${name:-${tag}}"
  strip_legacy=0
  if [[ -n "${eol}" ]]; then
    strip_legacy=1
    if [[ "${new_name}" != *" (LTS)" ]]; then
      new_name="${new_name} (LTS)"
    fi
  fi
  banner="$(bannerBlock "${tag}" "${minor}" "${newest}" "${eol}")"
  new_body="$(stripBanners "${strip_legacy}" <<< "${body}")"
  # Join with a blank line only when there are notes below the banner, so an empty body compares equal next run
  if [[ -n "${banner}" ]] && [[ -n "${new_body}" ]]; then
    new_body="${banner}"$'\n\n'"${new_body}"
  elif [[ -n "${banner}" ]]; then
    new_body="${banner}"
  fi

  if [[ "${new_name}" == "${name}" ]] && [[ "${new_body}" == "${body}" ]]; then
    continue
  fi
  changed=$((changed + 1))

  if [[ "${dry_run}" == "true" ]]; then
    echo "=== ${tag}"
    if [[ "${new_name}" != "${name}" ]]; then
      echo "title: '${name}' -> '${new_name}'"
    fi
    diff -u --label "${tag} (current)" --label "${tag} (new)" <(printf '%s\n' "${body}") <(printf '%s\n' "${new_body}") || true
    continue
  fi

  # GitHub treats an edit without make_latest as make_latest=true, so always pass it explicitly. Mark a release as
  # Latest only if it is still the highest stable release right now, so a newer release published during this run
  # can't lose the badge.
  latest_flag="--latest=false"
  if [[ "${tag}" == "$(highestRelease)" ]]; then
    latest_flag="--latest"
  fi
  notes="$(mktemp)"
  printf '%s\n' "${new_body}" > "${notes}"
  echo "Updating ${tag}"
  gh release edit "${tag}" --repo "${GITHUB_REPO}" --title "${new_name}" --notes-file "${notes}" "${latest_flag}" > /dev/null
  rm -f "${notes}"
done

# A release published during this run can make an earlier --latest edit stale, so finish by putting the Latest badge
# back on the highest stable release if anything moved it. Check again after each repair, in case a newer release was
# published in between.
if [[ "${dry_run}" != "true" ]]; then
  for _ in 1 2 3; do
    highest="$(highestRelease)"
    if [[ -z "${highest}" ]] || [[ "$(gh release view --repo "${GITHUB_REPO}" --json tagName --jq .tagName)" == "${highest}" ]]; then
      break
    fi
    echo "Marking ${highest} as Latest"
    gh release edit "${highest}" --repo "${GITHUB_REPO}" --latest > /dev/null
  done
fi

if [[ "${dry_run}" == "true" ]]; then
  echo "${changed} release(s) would change"
else
  echo "${changed} release(s) updated"
fi
