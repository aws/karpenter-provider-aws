#!/usr/bin/env bash
set -euo pipefail

SCRIPTPATH="$( cd "$(dirname "$0")" ; pwd -P )"

declare -a steps=(
  scripts/step-01-config.sh
  scripts/step-02-eksctl-cluster.sh
  scripts/step-03-tekton-controllers.sh
  scripts/step-04-aws-load-balancer.sh
  scripts/step-05-ebs-csi-driver.sh
  scripts/step-06-karpenter.sh
  scripts/step-07-provisioners.sh
  scripts/step-08-prometheus.sh
  scripts/step-09-kit-operator.sh
  scripts/step-10-tekton-permissions.sh
)

for step in "${steps[@]}"; do
  echo "👉 $step"
  source "${SCRIPTPATH}/$step"
done

echo "✅ Successfully setup test infrastructure"

