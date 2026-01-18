#!/usr/bin/env bash
# This script can be called via ./hack/update-json-library.sh` to update the
# go-json-experiment fork in the repo. 

# The HASH parameter may be set as part of the invocation to this script or
# the default hash from ./hack/JSON-EXPERIMENTAL-HASH will be used.
HASH="${HASH:-$(cat ./hack/JSON-EXPERIMENTAL-HASH)}"
GO_JSON_EXPERIMENT_DIR="pkg/internal/third_party/go-json-experiment/json"
rm -rf $GO_JSON_EXPERIMENT_DIR
git clone https://github.com/go-json-experiment/json $GO_JSON_EXPERIMENT_DIR
cd $GO_JSON_EXPERIMENT_DIR
git reset --hard $HASH
# If HASH was set to a keyword like HEAD, get the actual commit ID
HASH=$(git rev-parse HEAD)
cat <<EOF > ../README.md
Forked from: https://github.com/go-json-experiment/json
Commit Hash: $HASH

This internal fork exists to prevent dependency issues with go-json-experiment
until its API stabilizes.
EOF

# Remove git directories 
rm -rf ".git"
rm -rf ".github"
# Remove images
rm *.png
# Remove go.{mod|sum}
# NOTE: go-json-experiment has no go mod dependencies at the moment.
#       If this changes, the code will need to be updated.
rm go.mod go.sum
# Update references to point to the fork
find . -type f -name "*.go" -print0 | xargs -0 perl -pi -e "s#github.com/go-json-experiment/json#k8s.io/kube-openapi/${GO_JSON_EXPERIMENT_DIR}#g"
