#!/usr/bin/env bash
set -euo pipefail

if [ -z ${ENABLE_GIT_PUSH+x} ]; then
  ENABLE_GIT_PUSH=false
fi

echo "codegen running ENABLE_GIT_PUSH: ${ENABLE_GIT_PUSH}"

bandwidth() {
  GENERATED_FILE="pkg/providers/instancetype/zz_generated.bandwidth.go"
  NO_UPDATE=''
  SUBJECT="Bandwidth"

  go run hack/code/bandwidth_gen/main.go -- "${GENERATED_FILE}"

  GIT_DIFF=$(git diff --stat "${GENERATED_FILE}")
  checkForUpdates "${GIT_DIFF}" "${NO_UPDATE}" "${SUBJECT}" "${GENERATED_FILE}"
}

pricing() {
  declare -a PARTITIONS=(
    "aws"
    "aws-us-gov"
    # "aws-cn"
  )

  for partition in "${PARTITIONS[@]}"; do
    GENERATED_FILE="pkg/providers/pricing/zz_generated.pricing_${partition//-/_}.go"
    NO_UPDATE=" ${GENERATED_FILE} "$'| 4 ++--\n 1 file changed, 2 insertions(+), 2 deletions(-)'
    SUBJECT="Pricing"

    go run hack/code/prices_gen/main.go --partition "$partition" --output "$GENERATED_FILE"

    GIT_DIFF=$(git diff --stat "${GENERATED_FILE}")
    checkForUpdates "${GIT_DIFF}" "${NO_UPDATE}" "${SUBJECT} beside timestamps since last update" "${GENERATED_FILE}"
  done
}

vpcLimits() {
  GENERATED_FILE="pkg/providers/instancetype/zz_generated.vpclimits.go"
  NO_UPDATE=''
  SUBJECT="VPC Limits"

  go run hack/code/vpc_limits_gen/main.go -- \
    --url=https://raw.githubusercontent.com/aws/amazon-vpc-resource-controller-k8s/master/pkg/aws/vpc/limits.go \
    --output="${GENERATED_FILE}"

  GIT_DIFF=$(git diff --stat "${GENERATED_FILE}")
  checkForUpdates "${GIT_DIFF}" "${NO_UPDATE}" "${SUBJECT}" "${GENERATED_FILE}"
}

instanceTypeTestData() {
  GENERATED_FILE="pkg/fake/zz_generated.describe_instance_types.go"
  NO_UPDATE=''
  SUBJECT="Instance Type Test Data"

  go run hack/code/instancetype_testdata_gen/main.go --out-file ${GENERATED_FILE} \
    --instance-types t3.large,m5.large,m5.xlarge,p3.8xlarge,g4dn.8xlarge,c6g.large,inf1.2xlarge,inf1.6xlarge,trn1.2xlarge,m5.metal,dl1.24xlarge,m6idn.32xlarge,t4g.small,t4g.xlarge,t4g.medium

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
    echo "true" >/tmp/codegen-updates
    git add "${GENERATED_FILE}"
    if [[ $ENABLE_GIT_PUSH == true ]]; then
      gitCommitAndPush "${SUBJECT}"
    fi
  fi
}

gitOpenAndPullBranch() {
  git fetch origin
  git checkout codegen || git checkout -b codegen || true
}

gitCommitAndPush() {
  UPDATE_SUBJECT=$1
  git commit -m "CodeGen updates from AWS API for ${UPDATE_SUBJECT}"
  git push --set-upstream origin codegen
}

noUpdates() {
  UPDATE_SUBJECT=$1
  echo "No updates from AWS API for ${UPDATE_SUBJECT}"
}

if [[ $ENABLE_GIT_PUSH == true ]]; then
  gitOpenAndPullBranch
fi

bandwidth
pricing
vpcLimits
instanceTypeTestData
