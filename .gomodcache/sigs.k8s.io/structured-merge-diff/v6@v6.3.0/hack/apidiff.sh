#!/usr/bin/env bash

# Copyright 2024 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# apidiff.sh: Compare public API changes between revisions or directories using Git worktrees.

set -euo pipefail

# Usage Information
usage() {
    echo "Usage: $0 [-r <revision>] [-t <revision>] [directory ...]"
    echo "   -t <revision>: Report changes in code up to and including this revision."
    echo "                  Default is the current working tree instead of a revision."
    echo "   -r <revision>: Report changes in code added since this revision."
    echo "                  Default is the common base of master and HEAD."
    exit 1
}

# Default Values
TARGET_REVISION=""    # -t: Target revision
REFERENCE_REVISION="" # -r: Reference revision
TARGET_DIR="."        # Default directory to compare is current working directory
API_DIFF_TOOL="apidiff"
REF_API_SNAPSHOT="ref.api"
TGT_API_SNAPSHOT="target.api"
WORKTREES=()          # Track created worktrees for cleanup

# Parse Command-Line Arguments
while getopts ":t:r:" opt; do
    case ${opt} in
        t) TARGET_REVISION="$OPTARG" ;;
        r) REFERENCE_REVISION="$OPTARG" ;;
        \?) echo "Error: Invalid option -$OPTARG" >&2; usage ;;
        :) echo "Error: Option -$OPTARG requires an argument." >&2; usage ;;
    esac
done
shift $((OPTIND - 1))

# Remaining arguments are directories
if [ "$#" -ge 1 ]; then
    TARGET_DIR="$1"
fi

# Debug print to see all traps
trap -p

# Step 1: Create a temporary directory structure under _output
mkdir -p "_output"
TMP_DIR=$(mktemp -d "_output/apidiff.XXXXXX")
TMP_DIR=$(cd "${TMP_DIR}" && pwd)  # Convert to absolute path
TEMP_GOBIN="${TMP_DIR}/gobin"
TEMP_WORKTREES="${TMP_DIR}/worktrees"
mkdir -p "${TEMP_GOBIN}" "${TEMP_WORKTREES}"

# Single trap for cleanup
trap 'cleanup' EXIT

# shellcheck disable=SC2317
cleanup() {
    # Remove all created worktrees
    for worktree in "${WORKTREES[@]}"; do
        git worktree remove --force "$worktree"
    done
    # Remove temporary directory with all contents
    rm -rf "${TMP_DIR}"
}

# Update GOBIN to use temporary location
if ! command -v "${API_DIFF_TOOL}" &> /dev/null; then
    echo "Installing apidiff into ${TEMP_GOBIN}"
    GOBIN="${TEMP_GOBIN}" go install golang.org/x/exp/cmd/apidiff@latest
    # Add GOBIN to PATH
    export PATH=$PATH:${TEMP_GOBIN}
fi

# Set target revision: PULL_PULL_SHA > target > HEAD
if [ -z "${TARGET_REVISION}" ] && [ -n "${PULL_PULL_SHA:-}" ]; then
    TARGET_REVISION="${PULL_PULL_SHA}"
elif [ -z "${TARGET_REVISION}" ]; then
    TARGET_REVISION="HEAD"
fi

# Verify target commit exists
TARGET_REVISION="$(git rev-parse --verify "${TARGET_REVISION}")"

# Try to determine base revision if not explicitly set
if [ -z "${REFERENCE_REVISION}" ]; then
    if [ -n "${PULL_BASE_SHA:-}" ]; then
        # Use PULL_BASE_SHA directly as the base
        REFERENCE_REVISION="${PULL_BASE_SHA}"
    else
        # Fall back to merge-base with origin/master
        if ! REFERENCE_REVISION="$(git merge-base origin/master "${TARGET_REVISION}")"; then
            echo "Error: Could not determine base revision. Please configure git remote 'origin' or use -r explicitly." >&2
            exit 1
        fi
    fi
fi

# Verify base commit exists
REFERENCE_REVISION="$(git rev-parse --verify "${REFERENCE_REVISION}")"

# Step 2: Export API snapshot for the reference revision
REF_WORKTREE="${TEMP_WORKTREES}/ref"
echo "Creating Git worktree for reference revision: ${REFERENCE_REVISION}"
git worktree add "${REF_WORKTREE}" "${REFERENCE_REVISION}" --quiet
WORKTREES+=("${REF_WORKTREE}")
echo "Exporting API snapshot for reference revision..."
pushd "${REF_WORKTREE}" > /dev/null
"${API_DIFF_TOOL}" -m -w "${TMP_DIR}/${REF_API_SNAPSHOT}" "${TARGET_DIR}"
popd > /dev/null

# Step 3: Export API snapshot for the target revision
TGT_WORKTREE="${TEMP_WORKTREES}/target"
if [ -n "${TARGET_REVISION}" ]; then
    echo "Creating Git worktree for target revision: ${TARGET_REVISION}"
    git worktree add "${TGT_WORKTREE}" "${TARGET_REVISION}" --quiet
    WORKTREES+=("${TGT_WORKTREE}")
    TGT_PATH="${TGT_WORKTREE}"
else
    # If no target revision specified, compare with current working tree
    TGT_PATH="${TARGET_DIR}"
fi

echo "Exporting API snapshot for target revision..."
pushd "${TGT_PATH}" > /dev/null
"${API_DIFF_TOOL}" -m -w "${TMP_DIR}/${TGT_API_SNAPSHOT}" "${TARGET_DIR}"
popd > /dev/null

# Step 4: Compare the two API snapshots for incompatible changes
# Step 4: Compare the two API snapshots for changes
echo "Checking for API changes..."
# All changes
all_changes=$("${API_DIFF_TOOL}" -m "${TMP_DIR}/${REF_API_SNAPSHOT}" "${TMP_DIR}/${TGT_API_SNAPSHOT}" 2>&1 | grep -v -e "^Ignoring internal package" || true)
# Incompatible changes
incompatible_changes=$("${API_DIFF_TOOL}" -incompatible -m "${TMP_DIR}/${REF_API_SNAPSHOT}" "${TMP_DIR}/${TGT_API_SNAPSHOT}" 2>&1 | grep -v -e "^Ignoring internal package" || true)

# Print out results
echo
echo "API compatibility check completed."
res=0
if [ -n "$incompatible_changes" ]; then
    res=1
    echo "Incompatible API changes found!"
else
    echo "No incompatible API changes found."
fi
if [ -z "$all_changes" ]; then
    echo "No API changes found."
else
    echo "All API changes:"
    echo "$all_changes"
    echo
fi

exit ${res}