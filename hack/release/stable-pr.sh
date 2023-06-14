#!/usr/bin/env bash
set -euo pipefail

# updateKarpenterCoreGoMod bumps the karpenter-core go.mod to the release version so that the
# karpenter and karpenter-core release versions match
updateKarpenterCoreGoMod(){
  RELEASE_VERSION=$1
  if [[ $GITHUB_ACCOUNT != $MAIN_GITHUB_ACCOUNT ]]; then
    echo "not updating go mod for a repo other than the main repo"
    return
  fi
  go get -u "github.com/aws/karpenter-core@${RELEASE_VERSION}"
  cd test
  go get -u "github.com/aws/karpenter-core@${RELEASE_VERSION}"
  cd ..
  make tidy
}

# updateTektonPreUpgradeVersion updates the version that we use for the pre-upgrade E2E test suite
# so that we are constantly testing against the last minor version of Karpenter
updateTektonPreUpgradeVersion(){
  LAST_MINOR_VERSION=$(git tag --sort=committerdate | grep -v "v${RELEASE_VERSION_MAJOR}.${RELEASE_VERSION_MINOR}" | tail -1)
  TEKTON_RELEASE_LISTENER_PATH="tools/release-notification-listener/listener/tekton.go"
  GITHUB_ACTION_E2E_MATRIX_PATH=".github/workflows/e2e-matrix.yaml"

  # This command goes into the tekton release-listener file and replaces the preUpgradeVersion with the last minor version
  sed -i "s/preUpgradeVersion = \".*\"/preUpgradeVersion = \"$LAST_MINOR_VERSION\"/" "$TEKTON_RELEASE_LISTENER_PATH"
  sed -i "s/from_git_ref:.*/from_git_ref: $LAST_MINOR_VERSION/" "$GITHUB_ACTION_E2E_MATRIX_PATH"
}

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
source "${SCRIPT_DIR}/common.sh"

config

GIT_TAG=$(git describe --exact-match --tags || echo "none")
if [[ $(releaseType $GIT_TAG) != $RELEASE_TYPE_STABLE ]]; then
  echo "Not a stable release. Missing required git tag."
  exit 1
fi

versionData "$GIT_TAG"
updateKarpenterCoreGoMod "$GIT_TAG"
updateTektonPreUpgradeVersion

git config user.name "StableRelease"
git config user.email "StableRelease@users.noreply.github.com"
git remote set-url origin https://x-access-token:${GITHUB_TOKEN}@github.com/${GITHUB_REPO}
git config pull.rebase false

BRANCH_NAME="release-${GIT_TAG}"
git checkout -b "${BRANCH_NAME}"
git add go.mod
git add go.sum
git add test/go.mod
git add test/go.sum
git add website
git add charts/karpenter-crd/Chart.yaml
git add charts/karpenter/Chart.yaml
git add charts/karpenter/Chart.lock
git add charts/karpenter/values.yaml
git add charts/karpenter/README.md
git commit -m "Stable Release updates Release ${GIT_TAG}."
git push --set-upstream origin "${BRANCH_NAME}"