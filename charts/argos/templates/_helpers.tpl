{{/*
Expand the name of the chart.
*/}}
{{- define "argos.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "argos.fullname" -}}
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
{{- define "argos.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "argos.labels" -}}
helm.sh/chart: {{ include "argos.chart" . }}
{{ include "argos.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "argos.selectorLabels" -}}
app.kubernetes.io/name: {{ include "argos.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "argos.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "argos.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image reference.
*/}}
{{- define "argos.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

{{/*
CNPG cluster name.
*/}}
{{- define "argos.cnpgClusterName" -}}
{{ include "argos.fullname" . }}-pg
{{- end }}

{{/*
CNPG app secret name (auto-created by CNPG operator: <cluster>-app).
Contains "uri", "username", "password", "dbname", "host", "port".
*/}}
{{- define "argos.cnpgSecretName" -}}
{{ include "argos.cnpgClusterName" . }}-app
{{- end }}

{{/*
Database URL for external database mode.
*/}}
{{- define "argos.externalDatabaseUrl" -}}
{{ required "externalDatabase.url is required when cnpg.enabled=false" .Values.externalDatabase.url }}
{{- end }}

{{/*
Secret name for argosd credentials (non-database secrets: bootstrap password, OIDC).
*/}}
{{- define "argos.secretName" -}}
{{- if .Values.existingSecret }}
{{- .Values.existingSecret }}
{{- else }}
{{- include "argos.fullname" . }}
{{- end }}
{{- end }}
