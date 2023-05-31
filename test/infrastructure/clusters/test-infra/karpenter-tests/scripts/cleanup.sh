#!/usr/bin/env bash

# Clean up the CI account from failed CF stacks or failed test runs
# The script assumes the aws CLI command is authenticated AWS_REGION is set

# EXPIRATION_TIME is the exact time in UTC that resources expire and will be swept by the script
EXPIRATION_TIME=$(date -d 'now-12 hours' +%Y-%m-%dT%H:%M --utc)
echo "Expiration time for all resources is: $EXPIRATION_TIME"

# Grab all instances that are older than the EXPIRATION_TIME and then filter out the KITInfrastructure instances
old_instances=($(aws ec2 describe-instances --filters Name=instance-state-name,Values=running --query "Reservations[].Instances[?LaunchTime <= '$EXPIRATION_TIME'][]" | grep -v "kubernetes.io/cluster/KITInfrastructure" | jq -r '.[].InstanceId' | sort))
n_old_instances="${#old_instances[@]}"
echo "Removing ${n_old_instances} old instances"

if (( n_old_instances > 0 )); then
  aws ec2 terminate-instances --instance-ids ${old_instances}
fi

# Delete old stacks from the account
stacks=($(aws cloudformation list-stacks --stack-status-filter CREATE_COMPLETE ROLLBACK_FAILED ROLLBACK_COMPLETE DELETE_FAILED UPDATE_COMPLETE UPDATE_FAILED UPDATE_ROLLBACK_FAILED UPDATE_ROLLBACK_COMPLETE IMPORT_COMPLETE IMPORT_ROLLBACK_FAILED IMPORT_ROLLBACK_COMPLETE --query "StackSummaries[?CreationTime <= '$EXPIRATION_TIME']" | jq -r '.[].StackName' | grep 'karpenter-tests-*-*'))
n_stacks="${#stacks[@]}"
echo "Removing ${n_stacks} cloudformation stacks"

# Iterate through every line in the stack describe response
for stack in "${stacks[@]}"; do
  echo "Deleting Stack ${stack}"
  aws cloudformation delete-stack --no-cli-pager --stack-name "${stack}"
done

# Clean up old launch templates
launch_templates=($(aws ec2 describe-launch-templates --query "LaunchTemplates[?CreateTime <= '$EXPIRATION_TIME']" |  jq -r '.[].LaunchTemplateName' | grep "karpenter.k8s.aws"))
n_lts="${#launch_templates[@]}"
echo "Removing ${n_lts} launch templates"

# Iterate through every line in the launch template describe response
for lt in "${launch_templates[@]}"; do
  echo "Deleting LT ${lt}"
  aws ec2 delete-launch-template --no-cli-pager --launch-template-name "${lt}"
done