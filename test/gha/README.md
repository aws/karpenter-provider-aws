## Enabling Github Action Runs in Your AWS Account

1. Deploy the [Cloudformation stacks](https://github.com/aws/karpenter/tree/main/test/gha/cloudformation/README.md) into your account that enable Managed Prometheus and the Github Actions runner policies.
2. Remove the `if: github.repository == 'aws/karpenter'` lines from the top of the `e2e-matrix.yaml` file so that the jobs are allowed to run in other repositories besides Karpenter
3. Update the environment variables in the `e2e.yaml` and `e2e-upgrade.yaml` files so that they match the appropriate values for your account set-up. In particular, ensure that the following variable match your set-up exactly

    ```yaml
    env:
      AWS_REGION: <region>
      K8s_VERSION: 1.25
      ACCOUNT_ID: <account-id>
      ROLE_NAME: <github-actions-role-name>
      WORKSPACE_ID: <managed-prometheus-workspace-id>
    ```

4. Deploy the changes to a default branch and trigger a `workflow_dispatch` event to run the tests in GHA
5. [Optional] Update the `SLACK_WEBHOOK_URL` secret to reference a custom slack webhook url for publishing build notification messages into your build notification slack channel.