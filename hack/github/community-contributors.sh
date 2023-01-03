#!/usr/bin/env bash
set -euo pipefail

USAGE='Usage: '.$0.' [<previous release> <latest release>]'
TOKEN=${GITHUB_TOKEN:-$(cat $HOME/.git/token)}

if [ ! $# -gt 0 ];
then
    RELEASES=$(
        curl -s \
            -H "Accept: application/vnd.github+json" \
            -H "Authorization: token $TOKEN" \
            https://api.github.com/repos/aws/karpenter/releases
    )
    LATEST=$(echo $RELEASES | jq -r ".[0].tag_name")
    PREVIOUS=$(echo $RELEASES | jq -r ".[1].tag_name")
elif [ $# -eq 2 ];
then
    PREVIOUS=$1
    LATEST=$2
else
    echo $USAGE
    exit
fi

COMMITS=$(curl -s \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: token $TOKEN" \
    https://api.github.com/repos/aws/karpenter/compare/$PREVIOUS...$LATEST)

CONTRIBUTIONS=$(
    echo $COMMITS | jq -r '
    .commits
    | sort_by(.commit.author.date)
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
        .author != "StableRelease" and
        .author != "dependabot[bot]" and
        .author != "github-actions[bot]"
    )
' | jq -s
)

NUM_COMMUNITY_CONTRIBUTIONS=$(echo $COMMUNITY_CONTRIBUTIONS | jq length)

echo "Comparing $PREVIOUS and $LATEST"
echo "Community members contributed $NUM_COMMUNITY_CONTRIBUTIONS/$NUM_CONTRIBUTIONS ($(awk "BEGIN {print (100*$NUM_COMMUNITY_CONTRIBUTIONS/$NUM_CONTRIBUTIONS)}")%) commits"
echo $COMMUNITY_CONTRIBUTIONS | jq
