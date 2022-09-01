#!/bin/bash
set -eu -o pipefail

for i in $(
  find ./cmd ./pkg ./test ./hack -name "*.go"
); do
  if ! grep -q "Apache License" $i; then
    cat hack/license-boilerplate.txt $i >$i.new && mv $i.new $i
  fi
done
