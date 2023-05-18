#!/bin/bash

declare -a steps=(
  step01-config.sh
  step16-cleanup.sh
)

for step in "${steps[@]}"; do
  echo "$step"
  source $step
done
