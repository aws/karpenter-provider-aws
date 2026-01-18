#!/usr/bin/env bash
# This script uses helm to install prometheus

set -euo pipefail

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts

# create monitoring namespace
kubectl create ns monitoring || true
kubectl label ns monitoring scrape=enabled --overwrite=true

helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
  -n monitoring \
  -f ./.github/actions/install-prometheus/values.yaml \
  --set "kubelet.serviceMonitor.cAdvisorRelabelings[0].targetLabel=metrics_path" \
  --set "kubelet.serviceMonitor.cAdvisorRelabelings[0].action=replace" \
  --set "kubelet.serviceMonitor.cAdvisorRelabelings[0].sourceLabels[0]=__metrics_path__" \
  --wait