#!/usr/bin/env bash
# Count test flakes in karpenter PRs using GraphQL for efficiency
# Flake = matrix run with at least 1 failure AND at least 1 success

set -euo pipefail

DAYS=${1:-7}
REPOS=("kubernetes-sigs/karpenter" "aws/karpenter-provider-aws")
SINCE=$(date -v-${DAYS}d +%Y-%m-%dT00:00:00Z 2>/dev/null || date -d "$DAYS days ago" +%Y-%m-%dT00:00:00Z)

for REPO in "${REPOS[@]}"; do
    OWNER="${REPO%/*}"
    NAME="${REPO#*/}"
    echo "=== $REPO (since ${SINCE%T*}) ==="
    
    result=$(gh api graphql -f query='
    query($owner: String!, $name: String!) {
      repository(owner: $owner, name: $name) {
        pullRequests(first: 50, states: [OPEN, MERGED, CLOSED], orderBy: {field: UPDATED_AT, direction: DESC}) {
          nodes {
            number
            updatedAt
            commits(last: 1) {
              nodes {
                commit {
                  checkSuites(first: 5) {
                    nodes {
                      workflowRun { databaseId }
                      checkRuns(first: 10) {
                        nodes { name conclusion }
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }' -f owner="$OWNER" -f name="$NAME")

    flake_count=0
    flaky_pr_count=0
    total_prs=0

    while IFS=$'\t' read -r pr_num updated_at check_suites; do
        [[ -z "$pr_num" ]] && continue
        [[ "$updated_at" < "$SINCE" ]] && continue
        ((total_prs++))
        
        flake_lines=()
        while IFS=$'\t' read -r run_id check_runs; do
            [[ -z "$run_id" || "$run_id" == "null" ]] && continue
            
            failed=$(echo "$check_runs" | jq '[.[] | select(.name | test("presubmit|ci-test"; "i")) | select(.conclusion == "FAILURE")] | length')
            passed=$(echo "$check_runs" | jq '[.[] | select(.name | test("presubmit|ci-test"; "i")) | select(.conclusion == "SUCCESS")] | length')
            total=$((failed + passed))
            
            # Flake: at least 1 failure AND at least 1 success
            if [[ "$failed" -ge 1 && "$passed" -ge 1 ]]; then
                failed_names=$(echo "$check_runs" | jq -r '.[] | select(.name | test("presubmit|ci-test"; "i")) | select(.conclusion == "FAILURE") | .name' | tr '\n' ',' | sed 's/,$//')
                flake_lines+=("    └─ [$failed/$total failed] https://github.com/$REPO/actions/runs/$run_id ($failed_names)")
            fi
        done < <(echo "$check_suites" | jq -r '.[] | "\(.workflowRun.databaseId)\t\(.checkRuns.nodes | tojson)"')
        
        if [[ ${#flake_lines[@]} -gt 0 ]]; then
            echo "✗ #$pr_num https://github.com/$REPO/pull/$pr_num"
            printf '%s\n' "${flake_lines[@]}"
            ((flaky_pr_count++))
            ((flake_count+=${#flake_lines[@]}))
        else
            echo "✓ #$pr_num https://github.com/$REPO/pull/$pr_num"
        fi
    done < <(echo "$result" | jq -r '.data.repository.pullRequests.nodes[] | "\(.number)\t\(.updatedAt)\t\(.commits.nodes[0].commit.checkSuites.nodes | tojson)"')

    pct=$(awk "BEGIN {printf \"%.1f\", ($total_prs > 0) ? $flaky_pr_count * 100 / $total_prs : 0}")
    echo "Summary: $flaky_pr_count/$total_prs PRs with flakes ($pct%) ($flake_count total)"
    echo ""
done
