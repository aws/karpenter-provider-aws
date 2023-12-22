# Github Workflows Security Notice

Writing security workflows that can be accessed by third parties outside of your repository is inherently dangerous. There is a full list of vulnerabilities that you can subject yourself to when you enable external users to interact with your workflows. These vulnerabilities are well-described here: https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions as well as detail on how to mitigate these risks.

As a rule-of-thumb within the Karpenter workflows, we have chosen to always assign any input that _might_ come from a user in either a Github workflow or a composite action into environment variables any we are using a bash or javascript script as a step in the workflow or action. An example of this can be seen below:

```yaml
- name: Save info about the review comment as an artifact for other workflows that run on workflow_run to download them
  env:
    # We store these values in environment variables to avoid bash script injection
    # Specifically, it's important that we do this for github.event.review.body since this is user-controlled input
    # https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions
    REVIEW_BODY: ${{ github.event.review.body }}
    PULL_REQUEST_NUMBER: ${{ github.event.pull_request.number }}
    COMMIT_ID: ${{ github.event.review.commit_id }}
  run: |
    mkdir -p /tmp/artifacts
    { echo "$REVIEW_BODY"; echo "$PULL_REQUEST_NUMBER"; echo "$COMMIT_ID"; } >> /tmp/artifacts/metadata.txt
    cat /tmp/artifacts/metadata.txt
```

Note that, when you are writing Github workflows or composite actions to ensure to follow this code-style to reduce the attack surface could result from attempted script injection to the workflows.