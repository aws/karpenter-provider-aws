aws iam attach-role-policy --role-name KarpenterInstanceNodeRole \
    --policy-arn arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy

aws iam attach-role-policy --role-name KarpenterInstanceNodeRole \
    --policy-arn arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy

aws iam attach-role-policy --role-name KarpenterInstanceNodeRole \
    --policy-arn arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly

aws iam attach-role-policy --role-name KarpenterInstanceNodeRole \
    --policy-arn arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore
