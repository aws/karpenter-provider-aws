# Add the SQS and SSM VPC endpoints if we are creating a private cluster
# We need to grab all of the VPC details for the cluster in order to add the endpoint
# Add inbound rules for codeBuild security group, create temporary access entry
VPC_CONFIG=$(aws eks describe-cluster --name "$CLUSTER_NAME" --query "cluster.resourcesVpcConfig")
VPC_ID=$(echo "$VPC_CONFIG" | jq .vpcId -r)
echo CLUSTER_VPC_ID="$VPC_ID" >> "$GITHUB_ENV"
SUBNET_IDS=($(echo "$VPC_CONFIG" | jq '.subnetIds | join(" ")' -r))
SHARED_NODE_SG=$((aws ec2 describe-security-groups --filters Name=tag:aws:cloudformation:stack-name,Values=eksctl-"$CLUSTER_NAME"-cluster Name=tag:aws:cloudformation:logical-id,Values=ClusterSharedNodeSecurityGroup --query "SecurityGroups[0]") | jq .GroupId -r)
eks_cluster_sg=$((aws ec2 describe-security-groups --filters Name=tag:aws:eks:cluster-name,Values="$CLUSTER_NAME"  --query "SecurityGroups[0]") | jq .GroupId -r)
echo EKS_CLUSTER_SG="$eks_cluster_sg" >> "$GITHUB_ENV"

for SERVICE in "com.amazonaws.$REGION.ssm" "com.amazonaws.$REGION.eks" "com.amazonaws.$REGION.sqs"; do
  aws ec2 create-vpc-endpoint \
    --vpc-id "${VPC_ID}" \
    --vpc-endpoint-type Interface \
    --service-name "${SERVICE}" \
    --subnet-ids "${SUBNET_IDS[@]}" \
    --security-group-ids "${eks_cluster_sg}" \
    --tag-specifications "ResourceType=vpc-endpoint,Tags=[{Key=testing/type,Value=e2e},{Key=testing/cluster,Value=$CLUSTER_NAME},{Key=github.com/run-url,Value=https://github.com/$REPOSITORY/actions/runs/$RUN_ID},{Key=karpenter.sh/discovery,Value=$CLUSTER_NAME}]"
done

# VPC peering request from codebuild
aws ec2 create-vpc-peering-connection --vpc-id "${CODEBUILD_VPC}" --peer-vpc-id "${VPC_ID}" --tag-specifications "ResourceType=vpc-peering-connection,Tags=[{Key=testing/type,Value=e2e},{Key=testing/cluster,Value=$CLUSTER_NAME},{Key=github.com/run-url,Value=https://github.com/$REPOSITORY/actions/runs/$RUN_ID},{Key=karpenter.sh/discovery,Value=$CLUSTER_NAME}]"
vpc_peering_connection_id=$((aws ec2 describe-vpc-peering-connections --filters Name=accepter-vpc-info.vpc-id,Values="${VPC_ID}" --query "VpcPeeringConnections[0]") | jq .VpcPeeringConnectionId -r)
aws ec2 accept-vpc-peering-connection --vpc-peering-connection-id "${vpc_peering_connection_id}"
echo VPC_PEERING_CONNECTION_ID="$vpc_peering_connection_id" >> "$GITHUB_ENV"

# Modify route table for codebuild vpc
subnet_config=$(aws ec2 describe-subnets --filters Name=vpc-id,Values="${CODEBUILD_VPC}" Name=tag:aws-cdk:subnet-type,Values=Private --query "Subnets")
echo "$subnet_config" | jq '.[].SubnetId' -r |
while read -r subnet;
do
  ROUTE_TABLE_ID=$((aws ec2 describe-route-tables --filters Name=vpc-id,Values="${CODEBUILD_VPC}" Name=association.subnet-id,Values="$subnet" --query "RouteTables[0].RouteTableId") | jq -r)
  aws ec2 create-route --route-table-id "$ROUTE_TABLE_ID" --destination-cidr-block 192.168.0.0/16 --vpc-peering-connection-id "$vpc_peering_connection_id"
done


# Modify route table for cluster vpc
CLUSTER_ROUTE_TABLE=$(aws ec2 describe-route-tables --filters Name=vpc-id,Values="${VPC_ID}" Name=association.main,Values=false --query "RouteTables")
echo "$CLUSTER_ROUTE_TABLE" | jq '.[].RouteTableId' -r |
while read -r routeTableId;
do
  aws ec2 create-route --route-table-id $routeTableId --destination-cidr-block 10.0.0.0/16 --vpc-peering-connection-id "$vpc_peering_connection_id"
done

aws ec2 authorize-security-group-ingress --group-id "${SHARED_NODE_SG}" --protocol  all --source-group "${CODEBUILD_SG}"
aws ec2 authorize-security-group-ingress --group-id "${eks_cluster_sg}" --protocol  all --source-group "${CODEBUILD_SG}"

# There is currently no VPC private endpoint for the IAM API. Therefore, we need to
# provision and manage an instance profile manually.
aws iam create-instance-profile --instance-profile-name "KarpenterNodeInstanceProfile-${CLUSTER_NAME}" --tags Key=testing/cluster,Value="$CLUSTER_NAME"
aws iam add-role-to-instance-profile --instance-profile-name "KarpenterNodeInstanceProfile-${CLUSTER_NAME}" --role-name "KarpenterNodeRole-${CLUSTER_NAME}"

#Create private registry policy for pull through cache
MANAGED_NG=$(aws eks list-nodegroups --cluster-name "${CLUSTER_NAME}" --query nodegroups --output text)
node_role=$(aws eks describe-nodegroup --cluster-name "${CLUSTER_NAME}" --nodegroup-name "${MANAGED_NG}" --query nodegroup.nodeRole --output text | cut -d '/' -f 2)
echo NODE_ROLE="$node_role" >> "$GITHUB_ENV"
cat <<EOF >> policy.json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "PullThroughCache",
            "Effect": "Allow",
            "Action": [
               "ecr:BatchImportUpstreamImage",
               "ecr:CreateRepository"
            ],
            "Resource": [
                "arn:aws:ecr:$REGION:$ACCOUNT_ID:repository/ecr-public/*",
                "arn:aws:ecr:$REGION:$ACCOUNT_ID:repository/k8s/*",
                "arn:aws:ecr:$REGION:$ACCOUNT_ID:repository/quay/*"
            ]
        }
    ]
}
EOF
aws iam put-role-policy --role-name "${node_role}" --policy-name "PullThroughCachePolicy" --policy-document file://policy.json

# Use pull through cache to pull images that are needed for the tests to run as it requires a route to the internet for the first time
docker pull "$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/k8s/pause:3.6"
docker pull "$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/ecr-public/eks-distro/kubernetes/pause:3.2"
docker pull "$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/ecr-public/docker/library/alpine:latest"