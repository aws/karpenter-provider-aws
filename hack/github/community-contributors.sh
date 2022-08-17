#!/bin/bash -u

TOKEN=$(cat $HOME/.git/token)

RELEASES=$(
    curl -s \
        -H "Accept: application/vnd.github+json" \
        -H "Authorization: token $TOKEN" \
        https://api.github.com/repos/aws/karpenter/releases
)
LATEST=$(echo $RELEASES | jq -r ".[0].tag_name")
PREVIOUS=$(echo $RELEASES | jq -r ".[1].tag_name")

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
        .author != "Nick Tran" and
        .author != "Jason Deal" and
        .author != "Ryan Maleki" and
        .author != "Jonathan Innis" and
        .author != "Brandon Wagner" and
        .author != "Chris Negus" and
        .author != "Jim DeWaard" and
        .author != "dependabot[bot]"
    )
' | jq -s
)

NUM_COMMUNITY_CONTRIBUTIONS=$(echo $COMMUNITY_CONTRIBUTIONS | jq length)

echo "Comparing $PREVIOUS and $LATEST"
echo "Community members contributed $NUM_COMMUNITY_CONTRIBUTIONS/$NUM_CONTRIBUTIONS ($(awk "BEGIN {print (100*$NUM_COMMUNITY_CONTRIBUTIONS/$NUM_CONTRIBUTIONS)}")%) commits"
echo $COMMUNITY_CONTRIBUTIONS | jq
