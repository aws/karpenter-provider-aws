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
{{- printf "%s:%s@%s" .Values.controller.image.repository  (default (printf "v%s" .Chart.AppVersion) .Values.controller.image.tag) .Values.controller.image.digest }}
{{- else }}
{{- printf "%s:%s" .Values.controller.image.repository  (default (printf "v%s" .Chart.AppVersion) .Values.controller.image.tag) }}
{{- end }}
{{- end }}


{{/* Get PodDisruptionBudget API Version */}}
{{- define "karpenter.pdb.apiVersion" -}}
{{- if and (.Capabilities.APIVersions.Has "policy/v1") (semverCompare ">= 1.21-0" .Capabilities.KubeVersion.Version) -}}
{{- print "policy/v1" -}}
{{- else -}}
{{- print "policy/v1beta1" -}}
{{- end -}}
{{- end -}}

{{/*
Flatten Settings Map using "." syntax
*/}}
{{- define "flattenSettings" -}}
{{- $map := first . -}}
{{- $label := last . -}}
{{- range $key := (keys $map | uniq | sortAlpha) }}
  {{- $sublabel := $key -}}
  {{- $val := (get $map $key) -}}
  {{- if $label -}}
    {{- $sublabel = list $label $key | join "." -}}
  {{- end -}}
  {{/* Special-case "tags" since we want this to be a JSON object */}}
  {{- if eq $key "tags" -}}
    {{- if not (kindIs "invalid" $val) -}}
      {{- $sublabel | quote | nindent 2 }}: {{ $val | toJson | quote }}
    {{- end -}}
  {{- else if kindOf $val | eq "map" -}}
    {{- list $val $sublabel | include "flattenSettings" -}}
  {{- else -}}
  {{- if not (kindIs "invalid" $val) -}}
    {{- $sublabel | quote | nindent 2 -}}: {{ $val | quote }}
  {{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Flatten the stdout logging outputs from args provided
*/}}
{{- define "karpenter.controller.outputPathsList" -}}
{{ $paths := list -}}
{{- range .Values.controller.outputPaths -}}
    {{- $paths = printf "%s" . | quote  | append $paths -}}
{{- end -}}
{{ $paths | join ", " }}
{{- end -}}

{{/*
Flatten the stderr logging outputs from args provided
*/}}
{{- define "karpenter.controller.errorOutputPathsList" -}}
{{ $paths := list -}}
{{- range .Values.controller.errorOutputPaths -}}
    {{- $paths = printf "%s" . | quote  | append $paths -}}
{{- end -}}
{{ $paths | join ", " }}
{{- end -}}
