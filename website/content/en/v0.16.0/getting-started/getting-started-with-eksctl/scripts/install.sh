#!/bin/bash
set -euo pipefail #fail if one step fails

if [ "$#" -ne 1 ]
then
  echo "Missing required Karpenter version. Usage: install.sh v0.0.1"
  exit 1
fi

export KARPENTER_VERSION=$1

declare -a steps=(
  step01-config.sh
  step02-create-cluster.sh
  step03-iam-cloud-formation.sh
  step04-grant-access.sh
  step05-controller-iam.sh
  step06-add-spot-role.sh
  step07-install-helm-chart.sh
  step08-apply-helm-chart.sh
)

for step in "${steps[@]}"; do
  echo "$step"
  source $step
done
