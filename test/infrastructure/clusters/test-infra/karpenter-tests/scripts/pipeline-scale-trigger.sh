#!/usr/bin/env bash
set -euo pipefail

# Create a pipeline run for the scale test suite
cat <<EOF | kubectl create -f -
 apiVersion: tekton.dev/v1beta1
 kind: PipelineRun
 metadata:
   generateName: "scale-periodic-"
   namespace: karpenter-tests
 spec:
   timeouts:
    pipeline: "4h"
   params:
   - name: test-filter
     value: "Scale"
   - name: test-timeout
     value: "6h"
   pipelineRef:
     name: suite
   serviceAccountName: karpenter-tests
EOF