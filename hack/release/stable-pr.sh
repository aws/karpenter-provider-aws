#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
# shellcheck source=hack/release/common.sh
source "${SCRIPT_DIR}/common.sh"

git_tag="$(git describe --exact-match --tags || echo "none")"
if [[ "${git_tag}" != v* ]]; then
  echo "Not a stable release. Missing required git tag."
  exit 1
fi
version="${git_tag#v}"

git config user.name "StableRelease"
git config user.email "StableRelease@users.noreply.github.com"
git remote set-url origin "https://x-access-token:${GITHUB_TOKEN}@github.com/${GITHUB_REPO}"
git config pull.rebase false

branch_name="release-${git_tag}"

# Patch releases are cut from a release branch, so only the patch version references are updated on top of main.
# This block must exit before the end of the script: bash has already parsed it, but checking out main replaces this file.
if isPatchRelease "${version}"; then
  # The image digest is only known from the release build output in the tagged tree
  img_tag="$(yq '.controller.image.tag' charts/karpenter/values.yaml)"
  img_digest="$(yq '.controller.image.digest' charts/karpenter/values.yaml)"

  git fetch origin main
  git checkout --force -B "${branch_name}" origin/main

  short_version="${version%.*}"
  if [[ ! -d "website/content/en/v${short_version}" ]]; then
    echo "website/content/en/v${short_version} does not exist on main, skipping PR for ${git_tag}."
    echo "pr=false" >> "${GITHUB_OUTPUT:-/dev/null}"
    exit 0
  fi
  bumpPatchVersion "${version}" "website/content/en/v${short_version}"

  if isLatestMinor "${version}"; then
    bumpPatchVersion "${version}" "website/content/en/docs"
    editWebsiteConfig "${version}"
    for chart in karpenter karpenter-crd; do
      yq e -i ".version = \"${version}\"" "charts/${chart}/Chart.yaml"
      yq e -i ".appVersion = \"${version}\"" "charts/${chart}/Chart.yaml"
    done
    yq e -i ".controller.image.tag = \"${img_tag}\"" charts/karpenter/values.yaml
    yq e -i ".controller.image.digest = \"${img_digest}\"" charts/karpenter/values.yaml
    (cd charts/karpenter && helm-docs)
  fi

  git add website
  git add charts/karpenter-crd/Chart.yaml
  git add charts/karpenter/Chart.yaml
  git add charts/karpenter/values.yaml
  git add charts/karpenter/README.md
  if git diff --cached --quiet; then
    echo "No documentation changes for ${git_tag}, skipping PR."
    echo "pr=false" >> "${GITHUB_OUTPUT:-/dev/null}"
    exit 0
  fi
  git commit -m "docs: update for patch release ${git_tag}"
  git push --set-upstream origin "${branch_name}"
  echo "pr=true" >> "${GITHUB_OUTPUT:-/dev/null}"
  exit 0
fi

git checkout -b "${branch_name}"
git add go.mod
git add go.sum
git add hack/docs
git add website
git add charts/karpenter-crd/Chart.yaml
git add charts/karpenter/Chart.yaml
git add charts/karpenter/Chart.lock
git add charts/karpenter/values.yaml
git add charts/karpenter/README.md
git commit -m "Stable Release updates Release ${git_tag}."
git push --set-upstream origin "${branch_name}"
echo "pr=true" >> "${GITHUB_OUTPUT:-/dev/null}"
