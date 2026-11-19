#!/usr/bin/env bash
# update_cluster_config.sh <cluster> <update-cluster-config args...>
# Runs aws eks update-cluster-config and waits for the update to finish. EKS allows one update at a time, and a new
# cluster can still be running one from its creation (an addon or logging update), so ResourceInUseException is
# retried until the earlier update is done.
set -euo pipefail
CLUSTER_NAME="$1"
shift

for attempt in $(seq 1 30); do
  if update_id=$(aws eks update-cluster-config --name "$CLUSTER_NAME" "$@" --query update.id --output text 2> /tmp/update-cluster-config.err); then
    break
  fi
  grep -q ResourceInUseException /tmp/update-cluster-config.err || { cat /tmp/update-cluster-config.err >&2; exit 1; }
  echo "another update is in progress on $CLUSTER_NAME (attempt $attempt/30), retrying in 30s"
  update_id=""
  sleep 30
done
[[ -n "$update_id" ]] || { echo "::error::$CLUSTER_NAME still had an update in progress after 15 minutes"; exit 1; }

while status=$(aws eks describe-update --name "$CLUSTER_NAME" --update-id "$update_id" --query update.status --output text); [[ "$status" == InProgress ]]; do
  sleep 30
done
[[ "$status" == Successful ]] || { aws eks describe-update --name "$CLUSTER_NAME" --update-id "$update_id"; exit 1; }
aws eks wait cluster-active --name "$CLUSTER_NAME"
