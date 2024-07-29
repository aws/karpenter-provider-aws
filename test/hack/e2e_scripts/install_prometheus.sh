CHART=prometheus-community/kube-prometheus-stack
VALUES=./.github/actions/e2e/install-prometheus/values.yaml
ENABLED=true
ADDITIONAL_FLAGS=""
if (( "$PRIVATE_CLUSTER" == 'true' )); then
  CHART=$(find . -name kube-prometheus-stack*)
  ENABLED=false
  ADDITIONAL_FLAGS="--set prometheusOperator.admissionWebhooks.patch.image.registry=$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/k8s --set prometheusOperator.prometheusConfigReloader.image.registry=$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/quay --set prometheusOperator.image.registry=$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/quay --set kube-state-metrics.image.registry=$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/k8s --set alertmanager.alertmanagerSpec.image.registry=$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/quay --set prometheus.prometheusSpec.image.registry=$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/quay --set grafana.enabled=$ENABLED"
fi

helm upgrade --install prometheus "${CHART}" \
-n prometheus \
-f ${VALUES} \
--set prometheus.prometheusSpec.remoteWrite[0].url=https://aps-workspaces.$PROMETHEUS_REGION.amazonaws.com/workspaces/$WORKSPACE_ID/api/v1/remote_write \
--set prometheus.prometheusSpec.remoteWrite[0].sigv4.region=$PROMETHEUS_REGION \
--set prometheus.serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::$ACCOUNT_ID:role/prometheus-irsa-$CLUSTER_NAME" \
$ADDITIONAL_FLAGS \
--set "kubelet.serviceMonitor.cAdvisorRelabelings[0].targetLabel=metrics_path" \
--set "kubelet.serviceMonitor.cAdvisorRelabelings[0].action=replace" \
--set "kubelet.serviceMonitor.cAdvisorRelabelings[0].sourceLabels[0]=__metrics_path__" \
--set "kubelet.serviceMonitor.cAdvisorRelabelings[1].targetLabel=clusterName" \
--set "kubelet.serviceMonitor.cAdvisorRelabelings[1].replacement=$CLUSTER_NAME" \
--set "kubelet.serviceMonitor.cAdvisorRelabelings[2].targetLabel=gitRef" \
--set "kubelet.serviceMonitor.cAdvisorRelabelings[2].replacement=$(git rev-parse HEAD)" \
--set "kubelet.serviceMonitor.cAdvisorRelabelings[3].targetLabel=mostRecentTag" \
--set "kubelet.serviceMonitor.cAdvisorRelabelings[3].replacement=$(git describe --abbrev=0 --tags)" \
--set "kubelet.serviceMonitor.cAdvisorRelabelings[4].targetLabel=commitsAfterTag" \
--set "kubelet.serviceMonitor.cAdvisorRelabelings[4].replacement=\"$(git describe --tags | cut -d '-' -f 2)\"" \
--wait
