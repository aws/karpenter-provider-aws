#!/bin/bash
set -eu -o pipefail

cd website/themes/docsy
git apply ../../../hack/docsy.patch