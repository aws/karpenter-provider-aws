# Create the IAM role attached to Karpenter-launched nodes.
cat > /tmp/karpenter-node-trust.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": { "Service": "ec2.${AWS_PARTITION_DNS:-amazonaws.com}" },
    "Action": "sts:AssumeRole"
  }]
}
EOF

aws iam create-role \
  --role-name "KarpenterNodeRole-${CLUSTER_NAME}" \
  --assume-role-policy-document file:///tmp/karpenter-node-trust.json

for p in \
  AmazonEKS_CNI_Policy \
  AmazonEKSWorkerNodePolicy \
  AmazonEC2ContainerRegistryPullOnly \
  AmazonSSMManagedInstanceCore; do
  aws iam attach-role-policy \
    --role-name "KarpenterNodeRole-${CLUSTER_NAME}" \
    --policy-arn "arn:${AWS_PARTITION}:iam::aws:policy/$p"
done
