#!/usr/bin/env bash
set -euo pipefail

USAGE='Usage: '.$0.' [<previous release> <latest release>]'
TOKEN=$(gh auth token)

if [ ! $# -gt 0 ]; then
    RELEASES=$(
        curl -s \
            -H "Accept: application/vnd.github+json" \
            -H "Authorization: token $TOKEN" \
            https://api.github.com/repos/aws/karpenter-provider-aws/releases
    )
    LATEST=$(echo $RELEASES | jq -r ".[0].tag_name")
    PREVIOUS=$(echo $RELEASES | jq -r ".[1].tag_name")
elif [ $# -eq 2 ]; then
    PREVIOUS=$1
    LATEST=$2
else
    echo $USAGE
    exit
fi

COMMITS_PER_PAGE=500
RESPONSE=$(
    gh api \
        -H "Accept: application/vnd.github+json" \
        /repos/aws/karpenter-provider-aws/compare/$PREVIOUS...$LATEST?per_page=$COMMITS_PER_PAGE
)
TOTAL_COMMITS=$(echo $RESPONSE | jq -r ".total_commits")
PAGES=$(echo $((($TOTAL_COMMITS + $COMMITS_PER_PAGE - 1) / $COMMITS_PER_PAGE)))

COMMITS=""
for i in $(seq 1 $PAGES); do
    NEXT=$(
        gh api \
            -H "Accept: application/vnd.github+json" \
            /repos/aws/karpenter-provider-aws/compare/$PREVIOUS...$LATEST?per_page=$COMMITS_PER_PAGE\&page=$i | jq -r ".commits"
    )
    COMMITS=$(jq -s 'add' <(echo "$COMMITS") <(echo "$NEXT"))
done

CONTRIBUTIONS=$(
    echo $COMMITS | jq -r '
    sort_by(.commit.author.date)
    | .[].commit
    | {author: .author.name, message: (.message | split("\n")[0])}
' | jq -s
)
NUM_CONTRIBUTIONS=$(echo $CONTRIBUTIONS | jq length)

COMMUNITY_CONTRIBUTIONS=$(
    echo $CONTRIBUTIONS | jq -r '
    .[]
    | select(
        .author != "Ellis Tarn" and
        .author != "Suket Sharma" and
        .author != "Todd Neal" and
        .author != "Todd" and
        .author != "Nick Tran" and
        .author != "Jason Deal" and
        .author != "Ryan Maleki" and
        .author != "Jonathan Innis" and
        .author != "Amanuel Engeda" and
        .author != "Brandon Wagner" and
        .author != "Brandon" and
        .author != "Chris Negus" and
        .author != "Jim DeWaard" and
        .author != "Felix Zhe Huang" and
        .author != "Raghav Tripathi" and
        .author != "Justin Garrison" and
        .author != "Alex Kestner" and
        .author != "Geoffrey Cline" and
        .author != "Bill Rayburn" and
        .author != "Elton" and
        .author != "Prateek Gogia" and
        .author != "njtran" and
        .author != "dewjam" and
        .author != "suket22" and
        .author != "Jigisha Patil" and
        .author != "jigisha620" and
        .author != "nikmohan123" and
        .author != "StableRelease" and
        .author != "dependabot[bot]" and
        .author != "github-actions[bot]" and
        .author != "APICodeGen"
    )
' | jq -s
)

NUM_COMMUNITY_CONTRIBUTIONS=$(echo $COMMUNITY_CONTRIBUTIONS | jq length)

echo "Comparing $PREVIOUS and $LATEST"
echo $COMMUNITY_CONTRIBUTIONS | jq
echo "Community members contributed $NUM_COMMUNITY_CONTRIBUTIONS/$NUM_CONTRIBUTIONS ($(awk "BEGIN {print (100*$NUM_COMMUNITY_CONTRIBUTIONS/$NUM_CONTRIBUTIONS)}")%) commits"
