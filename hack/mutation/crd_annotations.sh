#!/usr/bin/env bash

# Add additional annotations variable to the CRDS

CRDS="charts/karpenter-crd/templates/*.yaml"
for CRD in $CRDS
do
  awk '{print} /  annotations:/ && !n {print "    {{- with .Values.additionalAnnotations }}\n      {{- toYaml . | nindent 4 }}\n    {{- end }}"; n++}' "$CRD" | sponge "$CRD"
done
