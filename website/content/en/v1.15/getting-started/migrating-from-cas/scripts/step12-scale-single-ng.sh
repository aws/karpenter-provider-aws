aws eks update-nodegroup-config --cluster-name "${CLUSTER_NAME}" \
    --nodegroup-name "${NODEGROUP}" \
    --scaling-config "minSize=2,maxSize=2,desiredSize=2"
