#!/usr/bin/env bash

# Clean up the CI account from failed CF stacks
# The script assumes the aws CLI command is authenticated AWS_REGION is set

all=$(aws ec2 describe-instances --filters Name=instance-state-name,Values=running | jq -r '.Reservations[] .Instances[] .InstanceId' | sort)
kit_infra=$(aws ec2 describe-instances --filters Name=tag-key,Values=kubernetes.io/cluster/KITInfrastructure Name=instance-state-name,Values=running | jq -r '.Reservations[] .Instances[] .InstanceId' | sort)
old_instances=$(echo -n "${all}\n${kit_infra}" | sort | uniq -c | tr -d " " | grep -v '^2' | grep -o 'i-[0-9a-z]\+' | tr '\n' ' ')

aws ec2 terminate-instances --instance-ids ${old_instances}

aws cloudformation list-stacks --stack-status-filter CREATE_COMPLETE ROLLBACK_FAILED ROLLBACK_COMPLETE DELETE_FAILED UPDATE_COMPLETE UPDATE_FAILED UPDATE_ROLLBACK_FAILED UPDATE_ROLLBACK_COMPLETE IMPORT_COMPLETE IMPORT_ROLLBACK_FAILED IMPORT_ROLLBACK_COMPLETE | jq -r '.StackSummaries[] .StackName' | grep 'karpenter-tests-*-*' | xargs -n 1 -t aws cloudformation delete-stack --stack-name


# Clean up old launch templates
launch_templates=$(aws ec2 describe-launch-templates |  jq -r '.LaunchTemplates[].LaunchTemplateName' | grep Karpenter-karpenter-tests)
n_lts=$(echo ${launch_templates} | wc -w)
echo "Removing ${n_lts} launch templates"
for lt in ${launch_templates}; do
  echo "Deleting LT ${lt}"
  aws ec2 delete-launch-template --no-cli-pager --launch-template-name ${lt}
done