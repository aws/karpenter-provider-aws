#!/usr/bin/env python3

import csv
import os
import sys
from operator import itemgetter
from typing import Union

# This script requires the python GitHub client:
# pip install PyGithub
from github import Github
from github.Repository import Repository

print('Getting popular feature requests...')

# To create a GitHub token, see below (the token doesn't need to include any scopes):
# https://help.github.com/en/github/authenticating-to-github/creating-a-personal-access-token-for-the-command-line
github = Github(os.environ.get('GH_TOKEN'))

issue_reaction_count: list[dict[str, Union[int, str]]] = []
PLUS_ONE_REACTION_STRINGS = ['+1', 'heart', 'hooray', 'rocket', 'eyes']
ISSUE_LABELS = ['feature']

repo: Repository = github.get_repo('aws/karpenter-provider-aws')
open_issues = repo.get_issues(state='open', labels=ISSUE_LABELS)
for issue in open_issues:
  # count unique +1s
  usernames: set[str] = set()
  plus_ones = 0
  for reaction in issue.get_reactions():
    username = reaction.user.login
    if reaction.content in PLUS_ONE_REACTION_STRINGS and username not in usernames:
      usernames.add(reaction.user.login)
      plus_ones += 1

  issue_reaction_count.append({
    'title': issue.title,
    'url': issue.html_url,
    'reactions': plus_ones
  })

issue_row_list = [['Title', 'Url', 'Plus Ones']]
for issue in sorted(issue_reaction_count, key=itemgetter('reactions'), reverse=True):
  issue_row_list.append([
    issue['title'],
    issue['url'],
    issue['reactions']
  ])

# Write CSV data to STDOUT, redirect to file to persist, e.g.
# ./hack/feature_request_reactions.py > "karpenter-feature-requests-$(date +"%Y-%m-%d").csv"
writer = csv.writer(sys.stdout)
writer.writerows(issue_row_list)

print('Done!')
