#!/usr/bin/env bash
set -euo pipefail

# GitHub marks the most recently published release as "Latest", so a patch for an older minor (a backport) takes the
# badge from the newest version. Re-mark the highest stable vX.Y.Z release as Latest.
latest=$(gh release list --repo "${GITHUB_REPO}" --exclude-drafts --exclude-pre-releases --limit 1000 --json tagName --jq '.[].tagName' |
  grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n 1 || true)
if [[ -z "${latest}" ]]; then
  echo "No stable releases found, leaving Latest unchanged"
  exit 0
fi
current=$(gh release view --repo "${GITHUB_REPO}" --json tagName --jq .tagName)
if [[ "${current}" == "${latest}" ]]; then
  echo "Latest is already ${latest}"
  exit 0
fi
echo "Marking ${latest} as Latest (was ${current})"
gh release edit "${latest}" --repo "${GITHUB_REPO}" --latest
