#!/bin/bash
set -eu -o pipefail

main() {
  local command=${1:-'--apply'}
  if [[ $command = "--apply" ]]; then
    echo "Installing Karpenter & dependencies.."
    apply
    echo "Installation complete!"
  elif [[ $command = "--delete" ]]; then
    echo "Uninstalling Karpenter & dependencies.."
    delete
    echo "Uninstallation complete!"
  else
    echo "Error: invalid argument: $command" >&2
    usage
    exit 22 # EINVAL
  fi
}

usage() {
  cat <<EOF
######################## USAGE ########################
hack/quick-install.sh          # Defaults to apply
hack/quick-install.sh --apply  # Creates all resources
hack/quick-install.sh --delete # Deletes all resources
#######################################################
EOF
}

delete() {
  helm delete karpenter || true
  helm delete cert-manager --namespace cert-manager || true
  helm delete kube-prometheus-stack --namespace monitoring || true
  kubectl delete namespace karpenter cert-manager monitoring || true
}

# If this fails you may have an old installation hanging around.
# `helm list -A`
# `helm delete <OLD_INSTALLATION>`
apply() {
  helm repo add jetstack https://charts.jetstack.io
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
  helm repo add karpenter https://awslabs.github.io/karpenter/charts
  helm repo update

  helm upgrade --install cert-manager jetstack/cert-manager \
    --version v1.2.0 \
    --create-namespace --namespace cert-manager \
    --set installCRDs=true \
    --atomic
  helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
    --version 13.7.1 \
    --create-namespace --namespace monitoring \
    --atomic
  helm upgrade --install karpenter karpenter/karpenter \
    --version 0.2.0 \
    --create-namespace --namespace karpenter \
    --atomic
}

usage
main "$@"
