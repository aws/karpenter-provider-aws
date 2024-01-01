#!/usr/bin/env bash
set -euo pipefail

if [ -z ${ENABLE_GIT_PUSH+x} ]; then
  ENABLE_GIT_PUSH=false
fi

echo "codegen running ENABLE_GIT_PUSH: ${ENABLE_GIT_PUSH}"

bandwidth() {
  GENERATED_FILE="pkg/providers/instancetype/zz_generated.bandwidth.go"
  SUBJECT="Bandwidth"

  go run hack/code/bandwidth_gen/main.go -- "${GENERATED_FILE}"

  checkForUpdates "${SUBJECT}" "${GENERATED_FILE}"
}

pricing() {
  declare -a PARTITIONS=(
    "aws"
    "aws-us-gov"
    # "aws-cn"
  )

  for partition in "${PARTITIONS[@]}"; do
    GENERATED_FILE="pkg/providers/pricing/zz_generated.pricing_${partition//-/_}.go"
    SUBJECT="Pricing"

    go run hack/code/prices_gen/main.go --partition "$partition" --output "$GENERATED_FILE"

    IGNORE_PATTERN="// generated at"
    checkForUpdates "${SUBJECT} beside timestamps since last update" "${GENERATED_FILE}" "${IGNORE_PATTERN}"
  done
}

vpcLimits() {
  GENERATED_FILE="pkg/providers/instancetype/zz_generated.vpclimits.go"
  SUBJECT="VPC Limits"

  go run hack/code/vpc_limits_gen/main.go -- \
    --url=https://raw.githubusercontent.com/aws/amazon-vpc-resource-controller-k8s/master/pkg/aws/vpc/limits.go \
    --output="${GENERATED_FILE}"

  checkForUpdates "${SUBJECT}" "${GENERATED_FILE}"
}

instanceTypeTestData() {
  GENERATED_FILE="pkg/fake/zz_generated.describe_instance_types.go"
  SUBJECT="Instance Type Test Data"

  go run hack/code/instancetype_testdata_gen/main.go --out-file ${GENERATED_FILE} \
    --instance-types t3.large,m5.large,m5.xlarge,p3.8xlarge,g4dn.8xlarge,c6g.large,inf1.2xlarge,inf1.6xlarge,trn1.2xlarge,m5.metal,dl1.24xlarge,m6idn.32xlarge,t4g.small,t4g.xlarge,t4g.medium

  checkForUpdates "${SUBJECT}" "${GENERATED_FILE}"
}

checkForUpdates() {
  SUBJECT=$1
  GENERATED_FILE=$2
  IGNORE_PATTERN=${3:-""}

  if [[ -z "$IGNORE_PATTERN" ]]; then
    GIT_DIFF=$(git diff --stat --ignore-blank-lines "${GENERATED_FILE}")
  else
    GIT_DIFF=$(git diff --stat --ignore-blank-lines --ignore-matching-lines="${IGNORE_PATTERN}" "${GENERATED_FILE}")
  fi

  echo "Checking git diff for updates..."
  if [[ -n "${GIT_DIFF}" ]]; then
    echo "$GIT_DIFF"
    git add "${GENERATED_FILE}"
    if [[ $ENABLE_GIT_PUSH == true ]]; then
      gitCommitAndPush "${SUBJECT}"
    fi
  else
    noUpdates "${SUBJECT}"
    git checkout "${GENERATED_FILE}"
  fi
}

gitOpenAndPullBranch() {
  git fetch origin
  git checkout -b codegen
}

gitCommitAndPush() {
  UPDATE_SUBJECT=$1
  git commit -m "CodeGen updates from AWS API for ${UPDATE_SUBJECT}"
  # Force push the branch since we might have left the branch around from the last codegen
  git push --set-upstream origin codegen --force
}

noUpdates() {
  UPDATE_SUBJECT=$1
  echo "No updates from AWS API for ${UPDATE_SUBJECT}"
}

if [[ $ENABLE_GIT_PUSH == true ]]; then
  gitOpenAndPullBranch
fi

echo "Updating bandwidth..."
bandwidth
echo "Updating pricing..."
pricing
echo "Updating VPC limits..."
vpcLimits
echo "Updating instance type data..."
instanceTypeTestData
echo "Finished codegen"
