helm repo add eks https://aws.github.io/eks-charts
helm repo update

helm upgrade --install --namespace aws-node-termination-handler --create-namespace \
  aws-node-termination-handler eks/aws-node-termination-handler \
    --set enableSpotInterruptionDraining="true" \
    --set enableRebalanceMonitoring="true" \
    --set enableRebalanceDraining="true" \
    --set enableScheduledEventDraining="true" \
    --set nodeSelector."karpenter\.sh/capacity-type"=spot
