aws iam create-instance-profile \
    --instance-profile-name "KarpenterNodeInstanceProfile-${ClusterName}"

aws iam add-role-to-instance-profile \
    --instance-profile-name "KarpenterNodeInstanceProfile-${ClusterName}" \
    --role-name "KarpenterNodeRole-${CLUSTER_NAME}"
