aws eks update-kubeconfig --name "$CLUSTER_NAME"

CHART="oci://$ECR_ACCOUNT_ID.dkr.ecr.$ECR_REGION.amazonaws.com/karpenter/snapshot/karpenter"
ADDITIONAL_FLAGS=""
if [[ "$PRIVATE_CLUSTER" == "true" ]]; then
  CHART="oci://$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/karpenter/snapshot/karpenter"
  ADDITIONAL_FLAGS="--set .Values.controller.image.repository=$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/karpenter/snapshot/controller --set .Values.controller.image.digest=\"\" --set .Values.postInstallHook.image.repository=$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/ecr-public/bitnami/kubectl --set .Values.postInstallHook.image.digest=\"\""
fi

helm upgrade --install karpenter "${CHART}" \
  -n kube-system \
  --version "0-$(git rev-parse HEAD)" \
  --set logLevel=debug \
  --set settings.isolatedVPC=${PRIVATE_CLUSTER} \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::$ACCOUNT_ID:role/karpenter-irsa-$CLUSTER_NAME" \
  $ADDITIONAL_FLAGS \
  --set settings.clusterName="$CLUSTER_NAME" \
  --set settings.interruptionQueue="$CLUSTER_NAME" \
  --set settings.featureGates.spotToSpotConsolidation=true \
  --set settings.featureGates.nodeRepair=true \
  --set settings.featureGates.reservedCapacity=true \
  --set controller.resources.requests.cpu=5 \
  --set controller.resources.requests.memory=3Gi \
  --set controller.resources.limits.cpu=5 \
  --set controller.resources.limits.memory=3Gi \
  --set serviceMonitor.enabled=true \
  --set serviceMonitor.additionalLabels.scrape=enabled \
  --set "serviceMonitor.endpointConfig.relabelings[0].targetLabel=clusterName" \
  --set "serviceMonitor.endpointConfig.relabelings[0].replacement=$CLUSTER_NAME" \
  --set "serviceMonitor.endpointConfig.relabelings[1].targetLabel=gitRef" \
  --set "serviceMonitor.endpointConfig.relabelings[1].replacement=$(git rev-parse HEAD)" \
  --set "serviceMonitor.endpointConfig.relabelings[2].targetLabel=mostRecentTag" \
  --set "serviceMonitor.endpointConfig.relabelings[2].replacement=$(git describe --abbrev=0 --tags)" \
  --set "serviceMonitor.endpointConfig.relabelings[3].targetLabel=commitsAfterTag" \
  --set "serviceMonitor.endpointConfig.relabelings[3].replacement=\"$(git describe --tags | cut -d '-' -f 2)\"" \
  --wait
