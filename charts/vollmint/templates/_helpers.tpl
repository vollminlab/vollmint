{{- define "vollmint.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "vollmint.fullname" -}}
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

{{- define "vollmint.labels" -}}
app: {{ include "vollmint.fullname" . }}
env: production
category: apps
app.kubernetes.io/name: {{ include "vollmint.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}

{{- define "vollmint.selectorLabels" -}}
app: {{ include "vollmint.fullname" . }}
app.kubernetes.io/name: {{ include "vollmint.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "vollmint.syncLabels" -}}
app: {{ include "vollmint.fullname" . }}-sync
env: production
category: apps
app.kubernetes.io/name: {{ include "vollmint.name" . }}-sync
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
