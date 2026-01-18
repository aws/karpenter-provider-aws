#!/usr/bin/env bash
set -euo pipefail

# This script ensures that we get all the directories that contain an "action.yaml" composite action
# and make sure that we have a dependabot entry for them. Currently, dependabot doesn't support wildcarding
# composite actions in a way that enables us to set a single entry for them. Instead, you need to grab all directories
# that contain actions that you want to auto-update and add an entry for each one in "dependabot.yaml"
# https://github.com/dependabot/dependabot-core/issues/6704

DIRS=($(find .github/actions -name "action.yaml" -type f -print0 | xargs -0 dirname | sort))
i=2 # Set the index to the starting index after all of the manually configured dependabot entries
for DIR in "${DIRS[@]}"; do
  i=$i dir=$DIR yq -i '.updates[env(i)] = {"package-ecosystem": "github-actions", "directory": env(dir), "schedule": {"interval": "weekly"}, "groups": {"action-deps": {"patterns": ["*"]}}}' .github/dependabot.yaml
  i=$((i+1))
done