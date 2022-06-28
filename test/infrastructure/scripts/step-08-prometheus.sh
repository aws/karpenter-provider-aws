helm repo add prometheus-community https://prometheus-community.github.io/helm-charts

helm upgrade --install prometheus --namespace monitoring --create-namespace \
  --set tolerations[0].key="CriticalAddonsOnly" \
  --set tolerations[0].operator="Exists" \
  --set coreDns.enabled=false \
  --set kubeProxy.enabled=false \
  --set kubeEtcd.enabled=false \
  --set alertmanager.enabled=false \
  --set kubeScheduler.enabled=false \
  --set kubeApiServer.enabled=false \
  --set kubeStateMetrics.enabled=false \
  --set kubeControllerManager.enabled=false \
  --set prometheus.serviceMonitor.selfMonitor=false \
  --set prometheusOperator.serviceMonitor.selfMonitor=false \
  prometheus-community/kube-prometheus-stack
