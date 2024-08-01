#!/usr/bin/env bash

# Add the conversion stanza to the CRD spec to enable conversion via webhook
yq eval '.spec.conversion = {"strategy": "Webhook", "webhook": {"conversionReviewVersions": ["v1beta1", "v1"], "clientConfig": {"service": {"name": "karpenter", "namespace": "kube-system", "port": 8443}}}}' -i pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml
yq eval '.spec.conversion = {"strategy": "Webhook", "webhook": {"conversionReviewVersions": ["v1beta1", "v1"], "clientConfig": {"service": {"name": "karpenter", "namespace": "kube-system", "port": 8443}}}}' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml
yq eval '.spec.conversion = {"strategy": "Webhook", "webhook": {"conversionReviewVersions": ["v1beta1", "v1"], "clientConfig": {"service": {"name": "karpenter", "namespace": "kube-system", "port": 8443}}}}' -i pkg/apis/crds/karpenter.sh_nodepools.yaml

# Update to the karpenter-crd charts

# Add the conversion stanza to the CRD spec to enable conversion via webhook
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
          namespace: {{ .Values.webhook.serviceNamespace }}
          port: {{ .Values.webhook.port }}
{{- end }}
" >>  charts/karpenter-crd/templates/karpenter.sh_nodepools.yaml

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
          namespace: {{ .Values.webhook.serviceNamespace }}
          port: {{ .Values.webhook.port }}
{{- end }}
" >>  charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml

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
          namespace: {{ .Values.webhook.serviceNamespace }}
          port: {{ .Values.webhook.port }}
{{- end }}
" >>  charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml