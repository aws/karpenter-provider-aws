#!/usr/bin/env bash
# Count test flakes in karpenter PRs by detecting workflow reruns that succeeded after failure

set -euo pipefail

DAYS=${1:-7}
REPOS=("kubernetes-sigs/karpenter" "aws/karpenter-provider-aws")
SINCE=$(date -v-${DAYS}d +%Y-%m-%d 2>/dev/null || date -d "$DAYS days ago" +%Y-%m-%d)

for REPO in "${REPOS[@]}"; do
    echo "=== $REPO (since $SINCE) ==="
    
    flake_count=0
    flaky_pr_count=0
    total_prs=0

    while read -r pr_num; do
        [[ -z "$pr_num" ]] && continue
        ((total_prs++))
        
        reruns=$(gh api "repos/$REPO/pulls/$pr_num/commits" -q '.[].sha' 2>/dev/null | while read -r sha; do
            gh api "repos/$REPO/actions/runs?head_sha=$sha" -q '.workflow_runs[] | select(.run_attempt > 1 and .conclusion == "success") | "\(.id) \(.name) attempt:\(.run_attempt)"' 2>/dev/null || true
        done | while read -r run_id rest; do
            [[ -z "$run_id" ]] && continue
            first=$(gh api "repos/$REPO/actions/runs/$run_id/attempts/1" -q '.conclusion' 2>/dev/null || true)
            [[ "$first" != "action_required" ]] && echo "$rest"
        done)
        
        if [[ -n "$reruns" ]]; then
            ((flaky_pr_count++))
            while IFS= read -r line; do
                echo "  https://github.com/$REPO/pull/$pr_num: $line"
                ((flake_count++))
            done <<< "$reruns"
        fi
    done < <(gh pr list --repo "$REPO" --state all --search "updated:>=$SINCE" --limit 500 --json number -q '.[].number')

    pct=$(awk "BEGIN {printf \"%.1f\", ($total_prs > 0) ? $flaky_pr_count * 100 / $total_prs : 0}")
    echo "Summary: $flaky_pr_count/$total_prs PRs with flakes ($pct%) ($flake_count reruns)"
    echo ""
done
