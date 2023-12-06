## Deploying the Github Actions CloudFormation Stack

### Deploying ManagedPrometheus and its Policy
```console
aws cloudformation deploy \
    --stack-name GithubActionsManagedPrometheus \
    --template-file prometheus_cloudformation.yaml \
    --capabilities CAPABILITY_NAMED_IAM
```

### Deploy Timestream
```console
aws cloudformation deploy \
   --stack-name GithubActionsTimestream \
   --template-file timestream_cloudformation.yaml \
   --parameter-overrides "DatabaseName=karpenterTesting" "TableName=scaleTestDurations" "SweeperTableName=sweeperCleanedResources" "ResourceCountTableName=resourceCount"
```

### [Optional] Deploying ManagedGrafana and its Policy
```console
aws cloudformation deploy \
    --stack-name GithubActionsManagedGrafana \
    --template-file grafana_cloudformation.yaml \
    --parameter-overrides "PrometheusWorkspaceID=<workspace-id>" \
    --capabilities CAPABILITY_NAMED_IAM
```

### Deploying IAM Policies and OIDC Provider

```console
aws cloudformation deploy --stack-name GithubActionsIAM \
    --template-file iam_cloudformation.yaml \
    --parameter-overrides "DatabaseName=karpenterTesting" "TableName=scaleTestDurations" "SweeperTableName=sweeperCleanedResources" "ResourceCountTableName=resourceCount" "Repository=aws/karpenter-provider-aws" Branches="*" "PrometheusWorkspaceID=<workspace-id>" Regions="us-east-2,us-west-2,..." \
    --capabilities CAPABILITY_NAMED_IAM
```

_Note: If deploying this cloudformation stack to reference back to your own repository, ensure you replace the `Repository` parameter override with your fully-qualified repository name in the format `<organization>/<repo-name>`. This parameter (along with the `Branches` parameter) tells the OIDC provider that is deployed for the action to reach out to, which branches and repos it should allow tokens to come from._