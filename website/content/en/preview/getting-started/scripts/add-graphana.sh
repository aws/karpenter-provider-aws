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
  step09-add-prometheus-graphana.sh
  step10-add-graphana-port-forward.sh
  step11-graphana-get-password.sh
)

i=0
for step in "${steps[@]}"; do
  ((i += 1))
  echo "Step $i"
  source $step
done
