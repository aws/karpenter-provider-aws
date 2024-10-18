#!/usr/bin/env bash

VERSION_START="$(cat charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml | yq '.spec.versions.[0] | line')"
VERSION_END="$(cat charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml | yq '.spec.versions.[1] | line')"
VERSION_END=$(($VERSION_END+1))

cat charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml | awk -v n=$VERSION_START 'NR==n {sub(/$/,"\n{{- if .Values.webhook.enabled }}")} 1' \
| awk -v n=$VERSION_END 'NR==n {sub(/$/,"\n{{- end }}")} 1' > ec2nc.yaml

cat ec2nc.yaml > charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml && rm ec2nc.yaml

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
" >>  charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml