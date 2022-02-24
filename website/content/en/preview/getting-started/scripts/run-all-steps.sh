#!/bin/bash

set -euo pipefail #fail if one step fails

declare -a steps=(
  step00-karpenter-version.sh
  step01-config.sh
  step02-create-cluster.sh
  step03-iam-cloud-formation.sh
  step04-grant-access.sh
  step05-controller-iam.sh
  step06-install-helm-chart.sh
  step07-apply-helm-chart.sh
)

i=0
for step in "${steps[@]}"
do
  echo "Step $i"
   source $step
   (( i += 1 ))
done
