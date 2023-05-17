## Deploying the Github Actions CloudFormation Stack

### Deploying ManagedPrometheus and its Policy
```console
aws cloudformation deploy \
    --stack-name GithubActionsManagedPrometheus \
    --template-file gha_prometheus_cloudformation.yaml \
    --capabilities CAPABILITY_NAMED_IAM
```

### [Optional] Deploying ManagedGrafana and its Policy
```console
aws cloudformation deploy \
    --stack-name GithubActionsManagedGrafana \
    --template-file gha_grafana_cloudformation.yaml \
    --parameter-overrides "GrafanaWorkspaceIDPMetadata=<saml-idp-metadata-contents>" "PrometheusWorkspaceID=<workspace-id>" \
    --capabilities CAPABILITY_NAMED_IAM
```

### Deploying Github Actions IAM Policies and OIDC Provider

```console
aws cloudformation deploy --stack-name GithubActionsIAM \
    --template-file gha_iam_cloudformation.yaml \
    --parameter-overrides "Repository=aws/karpenter" "PrometheusWorkspaceID=<workspace-id>" \
    --capabilities CAPABILITY_NAMED_IAM
```

_Note: If deploying this cloudformation stack to reference back to your own repository, ensure you replace the `Repository` parameter override with your fully-qualified repository name in the format `<organization>/<repo-name>`_