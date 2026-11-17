#!/usr/bin/env bash
# Tears down a private e2e cluster from the GitHub Actions runner. Every step is best-effort and finds its
# resources by name or tag, so it is safe after a partial setup or on a rerun.
set -uo pipefail

: "${CLUSTER_NAME:?}"
CODEBUILD_SG="${CODEBUILD_SG:-}"

# Delete the IRSA stacks first so the Karpenter controller loses its role and can't launch replacement nodes.
# eksctl would otherwise try to delete these service accounts through the unreachable Kubernetes API and stop.
for stack in $(aws cloudformation list-stacks \
  --query "StackSummaries[?StackStatus!='DELETE_COMPLETE' && starts_with(StackName, 'eksctl-${CLUSTER_NAME}-addon-iamserviceaccount-')].StackName" --output text); do
  aws cloudformation delete-stack --stack-name "${stack}" &&
    aws cloudformation wait stack-delete-complete --stack-name "${stack}"
done

# Without this instance profile Karpenter can't launch nodes, and KarpenterNodeRole can't be deleted while it's in it
aws iam remove-role-from-instance-profile --instance-profile-name "KarpenterNodeInstanceProfile-${CLUSTER_NAME}" --role-name "KarpenterNodeRole-${CLUSTER_NAME}"
aws iam delete-instance-profile --instance-profile-name "KarpenterNodeInstanceProfile-${CLUSTER_NAME}"

# Karpenter node ENIs block deleting the cluster VPC
instance_ids=$(aws ec2 describe-instances \
  --filters "Name=tag:eks:eks-cluster-name,Values=${CLUSTER_NAME}" Name=tag-key,Values=karpenter.sh/nodepool Name=instance-state-name,Values=pending,running,stopping,stopped \
  --query "Reservations[].Instances[].InstanceId" --output text)
if [[ -n "${instance_ids}" ]]; then
  # shellcheck disable=SC2086 # split the ID list into arguments
  aws ec2 terminate-instances --instance-ids ${instance_ids} > /dev/null &&
    aws ec2 wait instance-terminated --instance-ids ${instance_ids}
fi

# The nodegroup stack can't delete its NodeInstanceRole while the inline policy is attached
for stack in $(aws cloudformation list-stacks \
  --query "StackSummaries[?StackStatus!='DELETE_COMPLETE' && starts_with(StackName, 'eksctl-${CLUSTER_NAME}-nodegroup-')].StackName" --output text); do
  node_role=$(aws cloudformation describe-stack-resources --stack-name "${stack}" --logical-resource-id NodeInstanceRole \
    --query "StackResources[].PhysicalResourceId" --output text) && [[ -n "${node_role}" ]] &&
    aws iam delete-role-policy --role-name "${node_role}" --policy-name PullThroughCachePolicy
done

# VPC endpoint ENIs block deleting the cluster subnets
endpoints_query="VpcEndpoints[?State!='deleted' && State!='Deleted'].VpcEndpointId"
endpoint_ids=$(aws ec2 describe-vpc-endpoints --filters "Name=tag:testing/cluster,Values=${CLUSTER_NAME}" --query "${endpoints_query}" --output text)
if [[ -n "${endpoint_ids}" ]]; then
  # shellcheck disable=SC2086 # split the ID list into arguments
  aws ec2 delete-vpc-endpoints --vpc-endpoint-ids ${endpoint_ids} > /dev/null
  for _ in {1..30}; do
    [[ -z "$(aws ec2 describe-vpc-endpoints --filters "Name=tag:testing/cluster,Values=${CLUSTER_NAME}" --query "${endpoints_query}" --output text)" ]] && break
    sleep 10
  done
fi

cluster_vpc=$(aws cloudformation describe-stack-resources --stack-name "eksctl-${CLUSTER_NAME}-cluster" --logical-resource-id VPC \
  --query "StackResources[].PhysicalResourceId" --output text)
if [[ -n "${cluster_vpc}" && -n "${CODEBUILD_SG}" ]]; then
  for sg in $(aws ec2 describe-security-groups --filters "Name=vpc-id,Values=${cluster_vpc}" "Name=ip-permission.group-id,Values=${CODEBUILD_SG}" \
    --query "SecurityGroups[].GroupId" --output text); do
    aws ec2 revoke-security-group-ingress --group-id "${sg}" --protocol all --source-group "${CODEBUILD_SG}" > /dev/null
  done
fi

# Delete the peering routes first, otherwise the CodeBuild VPC keeps them as blackhole routes
for pcx in $(aws ec2 describe-vpc-peering-connections \
  --filters "Name=tag:testing/cluster,Values=${CLUSTER_NAME}" Name=status-code,Values=initiating-request,pending-acceptance,provisioning,active \
  --query "VpcPeeringConnections[].VpcPeeringConnectionId" --output text); do
  aws ec2 describe-route-tables --filters "Name=route.vpc-peering-connection-id,Values=${pcx}" \
    --query "RouteTables[].[RouteTableId, Routes[?VpcPeeringConnectionId=='${pcx}'].DestinationCidrBlock | [0]]" --output text |
  while read -r route_table cidr; do
    aws ec2 delete-route --route-table-id "${route_table}" --destination-cidr-block "${cidr}"
  done
  aws ec2 delete-vpc-peering-connection --vpc-peering-connection-id "${pcx}" > /dev/null
done

# --force pushes through Kubernetes API errors, since the private endpoint isn't reachable from here
eksctl delete cluster --name "${CLUSTER_NAME}" --force --wait --timeout 60m
