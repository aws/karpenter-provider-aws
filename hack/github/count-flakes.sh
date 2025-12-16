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
        
        pr_flakes=0
        while read -r sha; do
            [[ -z "$sha" ]] && continue
            while read -r run_id name attempt; do
                [[ -z "$run_id" ]] && continue
                for ((i=1; i<attempt; i++)); do
                    conclusion=$(gh api "repos/$REPO/actions/runs/$run_id/attempts/$i" -q '.conclusion' 2>/dev/null || true)
                    [[ "$conclusion" == "action_required" ]] && continue
                    echo "  https://github.com/$REPO/pull/$pr_num: $name attempt:$i"
                    ((pr_flakes++))
                done
            done < <(gh api "repos/$REPO/actions/runs?head_sha=$sha" -q '.workflow_runs[] | select(.run_attempt > 1 and .conclusion == "success") | "\(.id) \(.name) \(.run_attempt)"' 2>/dev/null || true)
        done < <(gh api "repos/$REPO/pulls/$pr_num/commits" -q '.[].sha' 2>/dev/null)
        
        if [[ $pr_flakes -gt 0 ]]; then
            ((flaky_pr_count++))
            ((flake_count+=pr_flakes))
        fi
    done < <(gh api --paginate "repos/$REPO/pulls?state=all&sort=updated&direction=desc&per_page=100" -q ".[] | select(.updated_at >= \"${SINCE}T00:00:00Z\") | .number")

    pct=$(awk "BEGIN {printf \"%.1f\", ($total_prs > 0) ? $flaky_pr_count * 100 / $total_prs : 0}")
    echo "Summary: $flaky_pr_count/$total_prs PRs with flakes ($pct%) ($flake_count reruns)"
    echo ""
done
