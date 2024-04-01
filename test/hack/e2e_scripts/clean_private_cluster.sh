#!/usr/bin/env bash

# Delete instance profile
aws iam remove-role-from-instance-profile --instance-profile-name "KarpenterNodeInstanceProfile-${CLUSTER_NAME}" --role-name "KarpenterNodeRole-${CLUSTER_NAME}"
aws iam delete-instance-profile --instance-profile-name "KarpenterNodeInstanceProfile-${CLUSTER_NAME}"

# Delete private registry policy for pull through cache
aws iam delete-role-policy --role-name "${NODE_ROLE}" --policy-name "PullThroughCachePolicy"

# Delete cluster
eksctl delete cluster --name "${CLUSTER_NAME}" --force

#Delete manually created VPC endpoints
endpoints=$(aws ec2 describe-vpc-endpoints --filters Name=vpc-id,Values="${CLUSTER_VPC_ID}" Name=tag:testing/cluster,Values="${CLUSTER_NAME}" --query "VpcEndpoints")
echo "$endpoints" | jq '.[].VpcEndpointId' -r |
while read -r endpointID;
do
  aws ec2 delete-vpc-endpoints --vpc-endpoint-ids "$endpointID"
  sleep 1
done

#Remove codebuild security group ingress from cluster security group
aws ec2 revoke-security-group-ingress --group-id "${EKS_CLUSTER_SG}" --protocol  all --source-group "${SG_CB}"

# Delete route table entry for cluster
subnet_config=$(aws ec2 describe-subnets --filters Name=vpc-id,Values="${VPC_CB}" Name=tag:aws-cdk:subnet-type,Values=Private --query "Subnets")
echo "$subnet_config" | jq '.[].SubnetId' -r |
while read -r subnet;
do
  ROUTE_TABLE_ID=$((aws ec2 describe-route-tables --filters Name=vpc-id,Values="${VPC_CB}" Name=association.subnet-id,Values="$subnet" --query "RouteTables[0].RouteTableId") | jq -r)
  aws ec2 delete-route --route-table-id "$ROUTE_TABLE_ID" --destination-cidr-block 192.168.0.0/16
done

# Delete VPC peering connection
aws ec2 delete-vpc-peering-connection --vpc-peering-connection-id "${VPC_PEERING_CONNECTION_ID}"