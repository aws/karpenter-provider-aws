#!/usr/bin/env python3

import csv
import os
import sys

# This script requires the python GitHub client:
# pip install PyGithub
from github import Github
from github.Repository import Repository

print('Getting popular issue labels...')

# To create a GitHub token, see below (the token doesn't need to include any scopes):
# https://help.github.com/en/github/authenticating-to-github/creating-a-personal-access-token-for-the-command-line
github = Github(os.environ.get('GH_TOKEN'))

issue_label_counts: dict[str, int] = {}
PLUS_ONE_REACTION_STRINGS = {'+1', 'heart', 'hooray', 'rocket', 'eyes'}

repo: Repository = github.get_repo('aws/karpenter-provider-aws')
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

# Write CSV data to STDOUT, redirect to file to persist, e.g.
# ./hack/label_issue_count.py > "karpenter-labels-$(date +"%Y-%m-%d").csv"
writer = csv.writer(sys.stdout)
writer.writerows(label_row_list)

print('Done!')
