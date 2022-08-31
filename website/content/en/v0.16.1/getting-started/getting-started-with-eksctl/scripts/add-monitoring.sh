#!/bin/bash
set -euo pipefail #fail if one step fails

if [ "$#" -ne 1 ]
then
  echo "Missing required Karpenter version. Usage: add-monitoring.sh v0.0.1"
  exit 1
fi

export KARPENTER_VERSION=$1

declare -a steps=(
  step01-config.sh
  step09-add-prometheus-grafana.sh
  step10-add-grafana-port-forward.sh
  step11-grafana-get-password.sh
)

for step in "${steps[@]}"; do
  echo "$step"
  source $step
done
