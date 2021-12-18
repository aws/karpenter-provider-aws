#!/usr/bin/env python

import csv
import os
from datetime import date
from operator import itemgetter
from typing import Dict, Set, Union

# This script requires the python GitHub client:
# pip install PyGithub
from github import Github
from github.Repository import Repository

print('Getting popular feature requests...')

# create a Github instance using an access token
github = Github(os.environ.get('GH_TOKEN'))
# to create a token
# see https://help.github.com/en/github/authenticating-to-github/creating-a-personal-access-token-for-the-command-line
# note, the token doesn't need to include any scopes

issue_reaction_count: list[dict[str, Union[int, str]]] = []
PLUS_ONE_REACTION_STRINGS = ['+1', 'heart', 'hooray', 'rocket', 'eyes']
ISSUE_LABELS = ['feature']

repo: Repository = github.get_repo('aws/karpenter')
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

with open(os.path.expanduser('karpenter-plus-ones-%s.csv' % (date.today())), 'w') as file:
  writer = csv.writer(file)
  writer.writerows(issue_row_list)

print('Done!')
