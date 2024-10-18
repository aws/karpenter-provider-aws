#!/usr/bin/env bash

VERSION_START="$(cat charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml | yq '.spec.versions.[0] | line')"
VERSION_END="$(cat charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml | yq '.spec.versions.[1] | line')"
VERSION_END=$(($VERSION_END+1))
TEMP=$(mktemp)

cat charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml | awk -v n=$VERSION_START 'NR==n {sub(/$/,"\n{{- if .Values.webhook.enabled }}")} 1' \
| awk -v n=$VERSION_END 'NR==n {sub(/$/,"\n{{- end }}")} 1' > $TEMP

cat $TEMP > charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml

echo "{{- if .Values.webhook.enabled }} 
  conversion:
    strategy: Webhook
    webhook:
      conversionReviewVersions:
        - v1beta1
        - v1
      clientConfig:
        service:
          name: {{ .Values.webhook.serviceName }}
          namespace: {{ .Values.webhook.serviceNamespace | default .Release.Namespace }}
          port: {{ .Values.webhook.port }}
{{- end }}
" >>  charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml