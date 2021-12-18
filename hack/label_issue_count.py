#!/usr/bin/env python

import csv
import os
from datetime import date
from typing import Dict, Set

# This script requires the python GitHub client:
# pip install PyGithub
from github import Github
from github.Repository import Repository

print('Getting popular issue labels...')

# create a Github instance using an access token
github = Github(os.environ.get('GH_TOKEN'))
# to create a token
# see https://help.github.com/en/github/authenticating-to-github/creating-a-personal-access-token-for-the-command-line
# note, the token doesn't need to include any scopes

issue_label_counts: dict[str, int] = {}
PLUS_ONE_REACTION_STRINGS = {'+1', 'heart', 'hooray', 'rocket', 'eyes'}

repo: Repository = github.get_repo('aws/karpenter')
open_issues = repo.get_issues(state='open')
for issue in open_issues:
  for label in issue.get_labels():
    if label.name not in issue_label_counts.keys():
      issue_label_counts[label.name] = 1
    else:
      issue_label_counts[label.name] += 1

label_row_list = [['Label', 'Issue Count']]
for label in sorted(issue_label_counts, key=issue_label_counts.get, reverse=True):
  label_row_list.append([label, issue_label_counts[label]])

with open(os.path.expanduser('karpenter-labels-%s.csv' % (date.today())), 'w') as file:
  writer = csv.writer(file)
  writer.writerows(label_row_list)

print('Done!')
