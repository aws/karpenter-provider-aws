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

## Credentialed jobs and pull request code

Some workflows have to publish something built from a pull request — a website preview, a snapshot image. Those deploys hold AWS credentials via OIDC and a write `GITHUB_TOKEN`, and a pull request can be opened by anyone, so they follow a few rules:

1. **Code from the pull request never runs in the job that holds the credentials.** The pull request is built in a separate job on the `pull_request` trigger, where a fork gets a fork-scoped token and a `refs/pull/N/merge` cache scope; the credentialed job only ships the artifact that job produced. Building pull request code from a `workflow_run` workflow instead would hand it the base repository's default-branch cache scope. Note that `actions/checkout`'s own fork-PR refusal does *not* catch this shape — for a run originating from `pull_request_review`, GitHub reports the base repository as `workflow_run.head_repository`, so the check passes — do not rely on it.
2. **Artifacts from those builds are hostile input.** Extract them with `./.github/actions/download-artifact`, which uses `unzip` (it strips `../` path components) and fails closed on non-regular files. Do not use `actions/download-artifact` for them: it extracts via `unzip-stream`, which joins the entry path onto the destination with no containment check. Uploads to S3 use `--no-follow-symlinks`, and privileged checkouts set `persist-credentials: false` so a symlink to `.git/config` cannot leak the token.
3. **The deploy is gated on an Environment.** The preview deploy job declares `environment: website-preview` so it is subject to that environment's protection rules, and it is pinned to a commit a repository member explicitly asked for by review comment.
4. **One producer workflow per command.** `workflow_run` selects by workflow *name*, so each `/karpenter <command>` has its own producer (`ApprovalComment`, `PreviewComment`) and a consumer can only be reached by its own command. This is structural; a body check inside each consumer is a convention that one consumer will eventually forget.