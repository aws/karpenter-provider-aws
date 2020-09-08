#!/bin/bash
set -eu -o pipefail

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

main() {
  local command=${1:-'--apply'}
  if [[ "$command" = "--usage" ]]; then
    usage
  elif [[ "$command" = "--apply" ]]; then
    apply
    echo Installation Complete!
  elif [[ "$command" = "--delete" ]]; then
    delete
    echo Uninstallation Complete!
  else
    echo "Error: invalid argument: $command"
    usage
    exit 1
  fi
}

usage() {
  cat <<EOF
######################## USAGE ########################
hack/quick-install.sh          # Defaults to apply
hack/quick-install.sh --usage  # Displays usage
hack/quick-install.sh --apply  # Creates all resources
hack/quick-install.sh --delete # Deletes all resources
#######################################################
EOF
}

delete() {
  helm uninstall cert-manager --namespace cert-manager || true
  helm uninstall prometheus --namespace prometheus || true
  make undeploy || true
}

apply() {
  helm repo add jetstack https://charts.jetstack.io
  helm repo add stable https://kubernetes-charts.storage.googleapis.com
  helm repo update

  certmanager
  prometheus
  make deploy
}

certmanager() {
  local cert_manager_dir=$TEMP_DIR/prometheus
  mkdir $cert_manager_dir
  helm upgrade --install cert-manager jetstack/cert-manager \
    --atomic \
    --create-namespace \
    --namespace cert-manager \
    --version v1.0.0 \
    --set installCRDs=true
}

prometheus() {
  local prometheus_dir=$TEMP_DIR/prometheus
  wget https://raw.githubusercontent.com/helm/charts/master/stable/prometheus/values.yaml --directory-prefix $prometheus_dir
  helm upgrade --install prometheus stable/prometheus \
    --atomic \
    --create-namespace \
    --namespace prometheus \
    --version 11.4.0 \
    --values $prometheus_dir/values.yaml
}

main "$@"
