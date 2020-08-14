# AWS Developer Guide

This file walks through any steps needed when developing Karpenter using AWS services.

## Set up EKS Cluster

_TODO_

## Set up ECR

If you plan on  using the AWS ECR and haven't yet set it up, you will want to do something like the following (which come from [these instructions](https://docs.aws.amazon.com/AmazonECR/latest/userguide/getting-started-cli.html)

```bash
# Replace the values in the following 2 lines:
AWS_ACCOUNT_ID=my-account-id
AWS_REGION=region
aws ecr get-login-password --region $AWS_ACCOUNT_ID | docker login --username AWS --password-stdin ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com
aws ecr create-repository --repository-name karpenter --image-scanning-configuration scanOnPush=true --region ${AWS_REGION}
```

You will then want to add the following to your shell's init script:

```bash
export KO_DOCKER_REPO=${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com
```
