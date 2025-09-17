#!/usr/bin/env bash
set -euo pipefail

if [ -z ${ENABLE_GIT_PUSH+x} ]; then
  ENABLE_GIT_PUSH=false
fi

echo "codegen running ENABLE_GIT_PUSH: ${ENABLE_GIT_PUSH}"

bandwidth() {
  GENERATED_FILE="pkg/providers/instancetype/zz_generated.bandwidth.go"

  go run hack/code/bandwidth_gen/main.go -- "${GENERATED_FILE}"

  checkForUpdates "${GENERATED_FILE}"
}

pricing() {
  declare -a PARTITIONS=(
    "aws"
    "aws-us-gov"
    # "aws-cn"
  )

  for partition in "${PARTITIONS[@]}"; do
    GENERATED_FILE="pkg/providers/pricing/zz_generated.pricing_${partition//-/_}.go"

    go run hack/code/prices_gen/main.go --partition "$partition" --output "$GENERATED_FILE"

    IGNORE_PATTERN="// generated at"
    checkForUpdates "${GENERATED_FILE}" "${IGNORE_PATTERN}"
  done
}

vpcLimits() {
  GENERATED_FILE="pkg/providers/instancetype/zz_generated.vpclimits.go"

  go run hack/code/vpc_limits_gen/main.go -- \
    --url=https://raw.githubusercontent.com/aws/amazon-vpc-resource-controller-k8s/master/pkg/aws/vpc/limits.go \
    --output="${GENERATED_FILE}"

  checkForUpdates "${GENERATED_FILE}"
}

instanceTypeTestData() {
  GENERATED_FILE="pkg/fake/zz_generated.describe_instance_types.go"

  go run hack/code/instancetype_testdata_gen/main.go --out-file ${GENERATED_FILE} \
    --instance-types t3.large,m5.large,m5.xlarge,p3.8xlarge,g4dn.8xlarge,c6g.large,inf2.xlarge,inf2.24xlarge,trn1.2xlarge,m5.metal,dl1.24xlarge,m6idn.32xlarge,t4g.small,t4g.xlarge,t4g.medium,g4ad.16xlarge,m7i-flex.large

  checkForUpdates "${GENERATED_FILE}"
}

# checkForUpdates is a helper function that takes in a file and an optional ignore pattern
# to determine if there is a diff between the previous iteration of the file and the newly generated data
# If it fines a difference between the new and the old file and the ENABLE_GIT_PUSH environment variable is set,
# it will push the updated file with an automatic commit to the "codegen" branch
# USAGE:
#   checkForUpdates "pkg/providers/pricing/zz_generated.pricing_aws.go" "// generated at"
checkForUpdates() {
  GENERATED_FILE=$1
  IGNORE_PATTERN=${2:-""}

  if [[ -z "$IGNORE_PATTERN" ]]; then
    GIT_DIFF=$(git diff --stat --ignore-blank-lines "${GENERATED_FILE}")
  else
    GIT_DIFF=$(git diff --stat --ignore-blank-lines --ignore-matching-lines="${IGNORE_PATTERN}" "${GENERATED_FILE}")
  fi

  echo "Checking git diff for updates..."
  if [[ -n "${GIT_DIFF}" ]]; then
    echo "$GIT_DIFF"
    if [[ $ENABLE_GIT_PUSH == true ]]; then
      gitCommitAndPush "${GENERATED_FILE}"
    fi
  else
    noUpdates "${GENERATED_FILE}"
    git checkout "${GENERATED_FILE}"
  fi
}

gitOpenAndPullBranch() {
  git fetch origin
  git checkout -b codegen
}

gitCommitAndPush() {
  GENERATED_FILE=$1
  git add "${GENERATED_FILE}"
  git commit -m "CodeGen updates from AWS API for ${GENERATED_FILE}"
  # Force push the branch since we might have left the branch around from the last codegen
  git push --set-upstream origin codegen --force
}

noUpdates() {
  GENERATED_FILE=$1
  echo "No updates from AWS API for ${GENERATED_FILE}"
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
