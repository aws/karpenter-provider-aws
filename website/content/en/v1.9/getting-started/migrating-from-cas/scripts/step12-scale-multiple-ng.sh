for NODEGROUP in $(aws eks list-nodegroups --cluster-name "${CLUSTER_NAME}" \
    --query 'nodegroups' --output text); do aws eks update-nodegroup-config --cluster-name "${CLUSTER_NAME}" \
    --nodegroup-name "${NODEGROUP}" \
    --scaling-config "minSize=1,maxSize=1,desiredSize=1"
done
