#!/bin/bash
set -eu -o pipefail

main() {
  COMMAND=${1:-'--apply'}
  if [ "$COMMAND" = "--usage" ]; then
    usage
  elif [ "$COMMAND" = "--apply" ]; then
    apply
  elif [ "$COMMAND" = "--delete" ]; then
    delete
  else
    echo "Error: invalid argument: $COMMAND"
    usage
    exit 1
  fi
}

usage() {
  echo "######################## USAGE ########################"
  echo "hack/quick-install.sh          # Defaults to apply"
  echo "hack/quick-install.sh --usage  # Displays usage"
  echo "hack/quick-install.sh --apply  # Creates all resources"
  echo "hack/quick-install.sh --delete # Deletes all resources"
  echo "#######################################################"
}

delete() {
  helm uninstall cert-manager --namespace cert-manager || true
  helm uninstall prometheus --namespace prometheus || true
  make undeploy || true
}

apply() {
  TEMP_DIR=$(mktemp -d)

  helm repo add jetstack https://charts.jetstack.io
  helm repo add stable https://kubernetes-charts.storage.googleapis.com
  helm repo update

  certmanager
  prometheus
  make deploy

  # Cleanup
  rm -r $TEMP_DIR
}

certmanager() {
  CERT_MANAGER_DIR=$TEMP_DIR/prometheus
  mkdir -p $CERT_MANAGER_DIR
  helm upgrade --install cert-manager jetstack/cert-manager \
    --atomic \
    --create-namespace \
    --namespace cert-manager \
    --version v1.0.0 \
    --set installCRDs=true
}

prometheus() {
  PROMETHEUS_DIR=$TEMP_DIR/prometheus
  wget https://raw.githubusercontent.com/helm/charts/master/stable/prometheus/values.yaml --directory-prefix $PROMETHEUS_DIR
  helm upgrade --install prometheus stable/prometheus \
    --atomic \
    --create-namespace \
    --namespace prometheus \
    --version 11.4.0 \
    --values $PROMETHEUS_DIR/values.yaml
}

main "$@"
