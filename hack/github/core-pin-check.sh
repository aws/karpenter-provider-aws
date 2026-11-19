#!/usr/bin/env bash
set -euo pipefail

# Usage: core-pin-check.sh <base-rev> <head-rev>
# Fails when go.mod replaces sigs.k8s.io/karpenter or pins a commit that isn't on core's branch for BASE_REF. Tags are
# skipped since release tags aren't on main. Warns on breaking core commits and on files a pin bump doesn't regenerate.
# Needs go, jq and gh.

base="$1"
head="$2"
base_ref="${BASE_REF:-main}"
summary="${GITHUB_STEP_SUMMARY:-/dev/null}"
module="sigs.k8s.io/karpenter"
core_repo="kubernetes-sigs/karpenter"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

# go mod edit -json parses go.mod like the go command, so comment, exclude and retract lines can't pass for the require
pin() {
  git show "$1:go.mod" > "${tmp}/go.mod"
  go mod edit -json "${tmp}/go.mod" | jq -c --arg m "${module}" \
    '{version: ([(.Require // [])[] | select(.Path == $m) | .Version] | first), replace: [(.Replace // [])[] | select(.Old.Path == $m)]}'
}
base_pin="$(pin "${base}")"
head_pin="$(pin "${head}")"
if [[ "${base_pin}" == "${head_pin}" ]]; then
  echo "The ${module} pin is unchanged"
  exit 0
fi
old="$(jq -r .version <<< "${base_pin}")"
new="$(jq -r .version <<< "${head_pin}")"
echo "The ${module} pin changed from ${base_pin} to ${head_pin}"
echo "## ${module} ${old} → ${new}" >> "${summary}"

failed=false
fail() {
  echo "::error::$*"
  echo "- :x: $*" >> "${summary}"
  failed=true
}
warn() {
  echo "::warning::$*"
  echo "- :warning: $*" >> "${summary}"
}
# On error gh prints the response body to stdout, so print just the HTTP status to stderr
api() {
  if ! gh api "$@" > "${tmp}/api.out" 2> "${tmp}/api.err"; then
    echo "GitHub API error ($(grep -oE 'HTTP [0-9]+' "${tmp}/api.err" | head -n 1 || echo "no HTTP status"))" >&2
    return 1
  fi
  cat "${tmp}/api.out"
}
# A pseudo-version ends in the commit's 12 character hash. Anything else is a tag
ref() {
  if [[ "$1" =~ [.-][0-9]{14}-([0-9a-f]{12})$ ]]; then echo "${BASH_REMATCH[1]}"; else echo "$1"; fi
}

if [[ "$(jq '.replace | length' <<< "${head_pin}")" != "0" ]]; then
  fail "go.mod replaces ${module}. Pin a commit from ${core_repo} instead"
fi

if [[ "${new}" == "null" ]]; then
  fail "go.mod doesn't require ${module}"
elif [[ "$(ref "${new}")" != "${new}" ]]; then
  commit="$(ref "${new}")"
  # Provider release branches are per patch (release-v1.15.0) or per minor (release-v1.13.x). Core's are per minor
  core_branch=main
  if [[ "${base_ref}" =~ ^release-v[0-9]+\.[0-9]+\.([0-9]+|x)$ ]]; then
    core_branch="${base_ref%.*}.x"
    if ! api "repos/${core_repo}/branches/${core_branch}" --silent 2> /dev/null; then
      echo "${core_repo} has no ${core_branch} branch, so the pin is checked against main"
      core_branch=main
    fi
  fi
  # "behind" or "identical" means the commit is reachable from the branch
  if ! status="$(api "repos/${core_repo}/compare/${core_branch}...${commit}" --jq .status 2>&1)"; then
    fail "Couldn't check whether ${commit} is on ${core_repo} ${core_branch}: ${status}"
  elif [[ "${status}" != "behind" && "${status}" != "identical" ]]; then
    fail "${module} ${new} pins ${commit}, which isn't on ${core_repo} ${core_branch} (${status}). Pin a merged commit"
  else
    echo "${commit} is on ${core_repo} ${core_branch}"
  fi
fi

if [[ "${old}" != "null" && "${new}" != "null" ]]; then
  if commits="$(api "repos/${core_repo}/compare/$(ref "${old}")...$(ref "${new}")" \
    --jq '.status, (.commits[] | .sha[0:12] + " " + (.commit.message | split("\n")[0]))' 2> "${tmp}/compare.err")"; then
    status="$(head -n 1 <<< "${commits}")"
    commits="$(tail -n +2 <<< "${commits}")"
    echo "The bump pulls in $(grep -c . <<< "${commits}" || true) core commits (${status}):"
    echo "${commits}"
    if [[ -n "${commits}" ]]; then
      {
        echo "### Core commits pulled in (${status})"
        sed -E 's/^([0-9a-f]+ [a-z]+(\([^)]*\))?!.*)$/**\1**/; s/^/- /' <<< "${commits}"
      } >> "${summary}"
    fi
    while read -r breaking; do
      warn "Breaking core change: ${breaking}"
    done < <(grep -E '^[0-9a-f]+ [a-z]+(\([^)]*\))?!' <<< "${commits}" || true)
  else
    warn "Couldn't list the core commits between ${old} and ${new}: $(cat "${tmp}/compare.err")"
  fi
fi

# Files a pin bump regenerates on its own
others="$(git diff --name-only "${base}" "${head}" | grep -Ev \
  -e '^go\.(mod|sum)$' \
  -e '^(pkg/apis/crds|charts/karpenter-crd/templates)/(karpenter\.sh|autoscaling\.x-k8s\.io)_[a-z]+\.yaml$' \
  -e '^charts/karpenter/templates/clusterrole-core\.yaml$' \
  -e '^website/content/en/preview/reference/(metrics|settings|annotations)\.md$' || true)"
if [[ -n "${others}" ]]; then
  # Only a warning since most bumps need provider changes
  warn "This PR bumps ${module} and changes other files. Check that they need the new core: $(paste -sd ' ' - <<< "${others}")"
fi

if [[ "${failed}" == "true" ]]; then
  exit 1
fi
