## Deploying the Github Actions CloudFormation Stack

### Deploying ManagedPrometheus and its Policy
```console
aws cloudformation deploy \
    --stack-name GithubActionsManagedPrometheus \
    --template-file prometheus_cloudformation.yaml \
    --capabilities CAPABILITY_NAMED_IAM
```

### Deploying Github Actions IAM Policies and OIDC Provider

```console
aws cloudformation deploy --stack-name GithubActionsIAM \
    --template-file gha_iam_cloudformation.yaml \
    --parameter-overrides "Repository=aws/karpenter" \
    --capabilities CAPABILITY_NAMED_IAM
```

_Note: If deploying this cloudformation stack to reference back to your own repository, ensure you replace the `Repository` parameter override with your fully-qualified repository name in the format `<organization>/<repo-name>`_