#!/bin/bash
set -euo pipefail #fail if one step fails

if [ "$#" -ne 1 ]
then
  echo "Missing required Karpenter version. Usage: setup-roles.sh v0.0.1"
  exit 1
fi

export KARPENTER_VERSION=$1
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)

declare -a steps=(
  step03-iam-cloud-formation.sh
  step04-grant-access.sh
  step05-controller-iam.sh
  step06-add-spot-role.sh
)

for step in "${steps[@]}"; do
  echo "$step"
  source "$SCRIPT_DIR/$step"
done
