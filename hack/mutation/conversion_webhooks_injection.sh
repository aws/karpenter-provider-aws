#!/usr/bin/env bash

# Update to the karpetner-crd charts

# Add Roll back version field NodePool 
yq eval '.spec.versions[0].storage = "updateV1Storage"' -i charts/karpenter-crd/templates/karpenter.sh_nodepools.yaml
yq eval '.spec.versions[1].storage = "updateV1beta1Storage"' -i charts/karpenter-crd/templates/karpenter.sh_nodepools.yaml

update=$(sed -e 's/updateV1Storage/{{ not .Values.rollbackToV1beta1 }}/g' -e 's/updateV1beta1Storage/{{ .Values.rollbackToV1beta1 }}/g'  charts/karpenter-crd/templates/karpenter.sh_nodepools.yaml)
rm charts/karpenter-crd/templates/karpenter.sh_nodepools.yaml
echo "$update" > charts/karpenter-crd/templates/karpenter.sh_nodepools.yaml

# Add Roll back version field NodePool 
yq eval '.spec.versions[0].storage = "updateV1Storage"' -i charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml
yq eval '.spec.versions[1].storage = "updateV1beta1Storage"' -i charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml

update=$(sed -e 's/updateV1Storage/{{ not .Values.rollbackToV1beta1 }}/g' -e 's/updateV1beta1Storage/{{ .Values.rollbackToV1beta1 }}/g'  charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml)
rm charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml
echo "$update" > charts/karpenter-crd/templates/karpenter.sh_nodeclaims.yaml

# Add Roll back version field EC2NodeClass 
yq eval '.spec.versions[0].storage = "updateV1Storage"' -i charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml
yq eval '.spec.versions[1].storage = "updateV1beta1Storage"' -i charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml

update=$(sed -e 's/updateV1Storage/{{ not .Values.rollbackToV1beta1 }}/g' -e 's/updateV1beta1Storage/{{ .Values.rollbackToV1beta1 }}/g'  charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml)
rm charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml
echo "$update" > charts/karpenter-crd/templates/karpenter.k8s.aws_ec2nodeclasses.yaml


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
