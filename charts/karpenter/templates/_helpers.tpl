{{/*
Expand the name of the chart.
*/}}
{{- define "karpenter.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "karpenter.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "karpenter.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "karpenter.labels" -}}
helm.sh/chart: {{ include "karpenter.chart" . }}
{{ include "karpenter.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.additionalLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "karpenter.selectorLabels" -}}
app.kubernetes.io/name: {{ include "karpenter.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "karpenter.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "karpenter.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Karpenter image to use
*/}}
{{- define "karpenter.controller.image" -}}
{{- if .Values.controller.image.digest }}
{{- printf "%s:%s@%s" .Values.controller.image.repository  (default (printf "%s" .Chart.AppVersion) .Values.controller.image.tag) .Values.controller.image.digest }}
{{- else }}
{{- printf "%s:%s" .Values.controller.image.repository  (default (printf "%s" .Chart.AppVersion) .Values.controller.image.tag) }}
{{- end }}
{{- end }}

{{/*
Karpenter controller container name
*/}}
{{- define "karpenter.controller.containerName" -}}
{{- .Values.controller.containerName | default "controller" -}}
{{- end -}}

{{/* Get PodDisruptionBudget API Version */}}
{{- define "karpenter.pdb.apiVersion" -}}
{{- if and (.Capabilities.APIVersions.Has "policy/v1") (semverCompare ">= 1.21-0" .Capabilities.KubeVersion.Version) -}}
{{- print "policy/v1" -}}
{{- else -}}
{{- print "policy/v1beta1" -}}
{{- end -}}
{{- end -}}

{{/*
Patch the label selector on an object
This template will add a labelSelector using matchLabels to the object referenced at _target if there is no labelSelector specified.
The matchLabels are created with the selectorLabels template.
This works because Helm treats dictionaries as mutable objects and allows passing them by reference.
*/}}
{{- define "karpenter.patchLabelSelector" -}}
{{- if not (hasKey ._target "labelSelector") }}
{{- $selectorLabels := (include "karpenter.selectorLabels" .) | fromYaml }}
{{- $_ := set ._target "labelSelector" (dict "matchLabels" $selectorLabels) }}
{{- end }}
{{- end }}

{{/*
Patch pod affinity
This template uses the patchLabelSelector template to add a labelSelector to pod affinity objects if there is no labelSelector specified.
This works because Helm treats dictionaries as mutable objects and allows passing them by reference.
*/}}
{{- define "karpenter.patchPodAffinity" -}}
{{- if (hasKey ._podAffinity "requiredDuringSchedulingIgnoredDuringExecution") }}
{{- range $term := ._podAffinity.requiredDuringSchedulingIgnoredDuringExecution }}
{{- include "karpenter.patchLabelSelector" (merge (dict "_target" $term) $) }}
{{- end }}
{{- end }}
{{- if (hasKey ._podAffinity "preferredDuringSchedulingIgnoredDuringExecution") }}
{{- range $weightedTerm := ._podAffinity.preferredDuringSchedulingIgnoredDuringExecution }}
{{- include "karpenter.patchLabelSelector" (merge (dict "_target" $weightedTerm.podAffinityTerm) $) }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Patch affinity
This template uses patchPodAffinity template to add a labelSelector to podAffinity & podAntiAffinity if one isn't specified.
This works because Helm treats dictionaries as mutable objects and allows passing them by reference.
*/}}
{{- define "karpenter.patchAffinity" -}}
{{- if (hasKey .Values.affinity "podAffinity") }}
{{- include "karpenter.patchPodAffinity" (merge (dict "_podAffinity" .Values.affinity.podAffinity) .) }}
{{- end }}
{{- if (hasKey .Values.affinity "podAntiAffinity") }}
{{- include "karpenter.patchPodAffinity" (merge (dict "_podAffinity" .Values.affinity.podAntiAffinity) .) }}
{{- end }}
{{- end }}

{{/*
Resolve the DRA settings to the values the controller receives, as JSON with the keys draNVIDIAGPU, draEFA, and
ignoreDRARequests. enableDRA turns on every DRA driver gate and stops ignoring DRA requests, so it's mutually exclusive
with the individual values: setting any of them alongside it fails the render, even to the value enableDRA implies.
Individually, enabling either gate still forces ignoreDRARequests off, since the gates contribute nothing while DRA
requests are ignored. The individual values default to null so that an explicit setting can be told apart from the
default. New DRA driver gates must be added to $gates.
*/}}
{{- define "karpenter.dra" -}}
{{- $settings := .Values.settings -}}
{{- $gates := list "draNVIDIAGPU" "draEFA" -}}
{{- if $settings.enableDRA -}}
{{- $set := list -}}
{{- range $gates -}}
{{- if not (kindIs "invalid" (index $settings.awsFeatureGates .)) -}}
{{- $set = append $set (printf "settings.awsFeatureGates.%s" .) -}}
{{- end -}}
{{- end -}}
{{- if not (kindIs "invalid" $settings.ignoreDRARequests) -}}
{{- $set = append $set "settings.ignoreDRARequests" -}}
{{- end -}}
{{- if $set -}}
{{- fail (printf "settings.enableDRA is mutually exclusive with %s: unset them, or unset settings.enableDRA" (join ", " $set)) -}}
{{- end -}}
{{- end -}}
{{- $resolved := dict -}}
{{- $anyGate := false -}}
{{- range $gates -}}
{{- $enabled := or $settings.enableDRA (index $settings.awsFeatureGates . | default false) -}}
{{- $_ := set $resolved . $enabled -}}
{{- $anyGate = or $anyGate $enabled -}}
{{- end -}}
{{- $ignore := ternary true $settings.ignoreDRARequests (kindIs "invalid" $settings.ignoreDRARequests) -}}
{{- $_ := set $resolved "ignoreDRARequests" (and (not $anyGate) $ignore) -}}
{{- toJson $resolved -}}
{{- end -}}

{{/*
Patch topology spread constraints
This template uses the patchLabelSelector template to add a labelSelector to topologySpreadConstraints if one isn't specified.
This works because Helm treats dictionaries as mutable objects and allows passing them by reference.
*/}}
{{- define "karpenter.patchTopologySpreadConstraints" -}}
{{- range $constraint := .Values.topologySpreadConstraints }}
{{- include "karpenter.patchLabelSelector" (merge (dict "_target" $constraint) $) }}
{{- end }}
{{- end }}
