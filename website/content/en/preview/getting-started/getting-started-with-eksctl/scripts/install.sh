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
  step09-add-prometheus-grafana.sh
  step10-add-grafana-port-forward.sh
  step11-grafana-get-password.sh
  step12-add-provisioner.sh
  step13-automatic-node-provisioning.sh
)

i=0
for step in "${steps[@]}"; do
  ((i += 1))
  echo "Step $i"
  source $step
done
