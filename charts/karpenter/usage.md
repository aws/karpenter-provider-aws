# Using the Helm chart

The examples here will demonstrate how to install Karpenter using the OCI (Open Containers
Initiative) [Helm](https://helm.sh/) chart.

A Helm installation is preferred because it installs both controller and webhook, and all the required
configurations.

For more information about installing and upgrading Karpenter, and different release types please
see [Upgrade Guide](https://karpenter.sh/preview/upgrade-guide/).

# Examples

Set the image of the Karpenter helm chart to the given tag that exists in
the [Karpenter Helm Chart Repo](https://gallery.ecr.aws/karpenter/karpenter):

```
export TAG="<INSERT_IMAGE_TAG>"
export CLUSTER_NAME="<INSERT_CLUSTER_NAME>"

export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export KARPENTER_IAM_ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
export CLUSTER_ENDPOINT="$(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.endpoint" --output text)"

helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version ${TAG} --namespace karpenter \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
  --set settings.aws.clusterName=${CLUSTER_NAME} \
  --set settings.aws.clusterEndpoint=${CLUSTER_ENDPOINT} \
  --set settings.aws.defaultInstanceProfile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
  --wait
```

## Snapshot Releases

Set the image to a snapshot release of a particular commit:

```
export COMMIT="<INSERT_COMMIT_HASH>"
export CLUSTER_NAME="<INSERT_CLUSTER_NAME>"
export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export KARPENTER_IAM_ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
export CLUSTER_ENDPOINT="$(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.endpoint" --output text)"

helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version v0-${COMMIT} --namespace karpenter \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
  --set settings.aws.clusterName=${CLUSTER_NAME} \
  --set settings.aws.clusterEndpoint=${CLUSTER_ENDPOINT} \
  --set settings.aws.defaultInstanceProfile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
  --wait
```

Example:

```
export COMMIT="e44fc30b0c670f34b32e8f7e34c3898ca18a1968"
export CLUSTER_NAME="<INSERT_CLUSTER_NAME>"

export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export KARPENTER_IAM_ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
export CLUSTER_ENDPOINT="$(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.endpoint" --output text)"

helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version v0-${COMMIT} --namespace karpenter \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
  --set settings.aws.clusterName=${CLUSTER_NAME} \
  --set settings.aws.clusterEndpoint=${CLUSTER_ENDPOINT} \
  --set settings.aws.defaultInstanceProfile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
  --wait
```

## Nightly Releases

Set the image to a snapshot release of a particular nightly release:

```
export DATE="<INSERT_DATE_IN_YYYYMMDD_FORMAT>"
export CLUSTER_NAME="<INSERT_CLUSTER_NAME>"

export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export KARPENTER_IAM_ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
export CLUSTER_ENDPOINT="$(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.endpoint" --output text)"

helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version v0-${DATE} --namespace karpenter \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
  --set settings.aws.clusterName=${CLUSTER_NAME} \
  --set settings.aws.clusterEndpoint=${CLUSTER_ENDPOINT} \
  --set settings.aws.defaultInstanceProfile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
  --wait
```

Example:

```
export DATE="20220715"
export CLUSTER_NAME="<INSERT_CLUSTER_NAME>"

export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export KARPENTER_IAM_ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
export CLUSTER_ENDPOINT="$(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.endpoint" --output text)"

helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version ${DATE} --namespace karpenter \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
  --set settings.aws.clusterName=${CLUSTER_NAME} \
  --set settings.aws.clusterEndpoint=${CLUSTER_ENDPOINT} \
  --set settings.aws.defaultInstanceProfile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
  --wait
```

## Troubleshooting

If Helm is showing an error when trying to install Karpenter helm charts:

1. Ensure you are using a newer Helm version, Helm started supporting OCI images since v3.8.0
2. Verify that the image you are trying to pull actually exists
   in [gallery.ecr.aws/karpenter](https://gallery.ecr.aws/karpenter)
3. Sometimes Helm generates a generic error, you can add the `--debug` switch to any of the helm commands in this doc
   for more verbose error messages
4. If you are getting a 403 forbidden error, you can try `docker logout public.ecr.aws` as
   explained [here](https://docs.aws.amazon.com/AmazonECR/latest/public/public-troubleshooting.html)

For additional troubleshooting, checkout our [troubleshooting docs](https://karpenter.sh/docs/troubleshooting/).