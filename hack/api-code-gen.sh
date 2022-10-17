#!/usr/bin/env bash
set -euo pipefail

pricing() {
  GENERATED_FILE="pkg/cloudproviders/aws/cloudprovider/zz_generated.pricing.go"
  NO_UPDATE=$' pkg/cloudproviders/aws/cloudprovider/zz_generated.pricing.go | 4 ++--\n 1 file changed, 2 insertions(+), 2 deletions(-)'
  SUBJECT="Pricing"

  go run hack/code/prices_gen.go -- "${GENERATED_FILE}"

  GIT_DIFF=$(git diff --stat "${GENERATED_FILE}")
  checkForUpdates "${GIT_DIFF}" "${NO_UPDATE}" "${SUBJECT} beside timestamps since last update" "${GENERATED_FILE}"
}

vpcLimits() {
  GENERATED_FILE="pkg/cloudproviders/aws/cloudprovider/zz_generated.vpclimits.go"
  NO_UPDATE=''
  SUBJECT="VPC Limits"

  go run hack/code/vpc_limits_gen.go -- \
    --url=https://raw.githubusercontent.com/aws/amazon-vpc-resource-controller-k8s/master/pkg/aws/vpc/limits.go \
    --output="${GENERATED_FILE}"

  GIT_DIFF=$(git diff --stat "${GENERATED_FILE}")
  checkForUpdates "${GIT_DIFF}" "${NO_UPDATE}" "${SUBJECT}" "${GENERATED_FILE}"
}

checkForUpdates() {
  GIT_DIFF=$1
  NO_UPDATE=$2
  SUBJECT=$3
  GENERATED_FILE=$4

  echo "Checking git diff for updates. ${GIT_DIFF}, ${NO_UPDATE}"
  if [[ "${GIT_DIFF}" == "${NO_UPDATE}" ]]; then
    noUpdates "${SUBJECT}"
    git checkout "${GENERATED_FILE}"
  else
    echo "true" >/tmp/api-code-gen-updates
    git add "${GENERATED_FILE}"
    gitCommitAndPush "${SUBJECT}"
  fi
}

gitOpenAndPullBranch() {
  git fetch origin
  git checkout api-code-gen || git checkout -b api-code-gen || true
}

gitCommitAndPush() {
  UPDATE_SUBJECT=$1
  git commit -m "APICodeGen updates from AWS API for ${UPDATE_SUBJECT}"
  git push --set-upstream origin api-code-gen
}

noUpdates() {
  UPDATE_SUBJECT=$1
  echo "No updates from AWS API for ${UPDATE_SUBJECT}"
}

gitOpenAndPullBranch
pricing
vpcLimits
