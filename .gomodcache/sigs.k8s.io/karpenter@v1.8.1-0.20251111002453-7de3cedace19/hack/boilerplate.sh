#!/bin/bash
set -eu -o pipefail

for i in $(
  find ./ -name "*.go"
); do
  if ! grep -q "Apache License" $i; then
    cat hack/boilerplate.go.txt $i >$i.new && mv $i.new $i
  fi
done
