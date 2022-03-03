#!/bin/bash
set -euo pipefail #fail if one step fails

declare -a steps=(
  step01-config.sh
  step14-deprovisioning.sh
  step16-cleanup.sh
)

i=0
for step in "${steps[@]}"; do
  ((i += 1))
  echo "Step $i"
  source $step
done
