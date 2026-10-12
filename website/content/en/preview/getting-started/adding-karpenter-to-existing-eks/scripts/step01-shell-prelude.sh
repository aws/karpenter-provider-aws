unset AWS_REGION                                    # avoid env shadowing
export AWS_PROFILE=<your-profile>                   # replace
export AWS_DEFAULT_REGION=<your-region>             # replace, e.g. us-east-1
export CLUSTER_NAME=<your-cluster-name>             # replace

# Confirm the resolved region matches your intent
aws configure list | grep region
