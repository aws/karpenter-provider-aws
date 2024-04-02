eksctl create iamidentitymapping \
--username system:node:{{EC2PrivateDNSName}} \
--cluster "$CLUSTER_NAME" \
--arn "arn:aws:iam::$ACCOUNT_ID:role/KarpenterNodeRole-$CLUSTER_NAME" \
--group system:bootstrappers \
--group system:nodes \
--group eks:kube-proxy-windows
