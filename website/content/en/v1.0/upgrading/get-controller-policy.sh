#!/usr/bin/env bash

sourceVersionCfn=$(mktemp)
versionTag=$([[ ${KARPENTER_VERSION} == v* ]] && echo "${KARPENTER_VERSION}" || echo "v${KARPENTER_VERSION}")
curl -fsSL https://raw.githubusercontent.com/aws/karpenter-provider-aws/${versionTag}/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml > ${sourceVersionCfn}

# Substitute the cloudformation templating strings for our environment variables
tmpfile=$(mktemp)
sed \
  -e 's/!Sub//g' \
  -e 's/${AWS::Partition}/${AWS_PARTITION}/g' \
  -e 's/${AWS::Region}/${AWS_REGION}/g' \
  -e 's/${AWS::AccountId}/${AWS_ACCOUNT_ID}/g' \
  -e 's/${ClusterName}/${CLUSTER_NAME}/g' \
  -e 's/${KarpenterInterruptionQueue.Arn}/arn:${AWS_PARTITION}:sqs:${AWS_REGION}:${AWS_ACCOUNT_ID}:${CLUSTER_NAME}/g' \
  -e 's/${KarpenterNodeRole.Arn}/arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role\/KarpenterNodeRole-${CLUSTER_NAME}/g' \
  "${sourceVersionCfn}" > "${tmpfile}" && mv "${tmpfile}" "${sourceVersionCfn}"

yq '.Resources.KarpenterControllerPolicy.Properties.PolicyDocument' ${sourceVersionCfn} | envsubst
