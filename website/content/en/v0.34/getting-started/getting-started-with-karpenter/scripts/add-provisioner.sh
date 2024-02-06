#!/bin/bash
set -euo pipefail #fail if one step fails

declare -a steps=(
  step01-config.sh
  step12-add-provisioner.sh
  step13-automatic-node-provisioning.sh
  step14-automatic-node-termination.sh
)

for step in "${steps[@]}"; do
  echo "$step"
  source $step
done
