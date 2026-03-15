{{- define "nullus.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nullus.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "nullus.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nullus.labels" -}}
helm.sh/chart: {{ include "nullus.chart" . }}
app.kubernetes.io/name: {{ include "nullus.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "nullus.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nullus.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "nullus.api.fullname" -}}
{{- printf "%s-api" (include "nullus.fullname" .) -}}
{{- end -}}

{{- define "nullus.web.fullname" -}}
{{- printf "%s-web" (include "nullus.fullname" .) -}}
{{- end -}}

{{- define "nullus.secretName" -}}
{{- printf "%s-secrets" (include "nullus.fullname" .) -}}
{{- end -}}

{{- define "nullus.postgresqlHost" -}}
{{- printf "%s-postgresql" (include "nullus.fullname" .) -}}
{{- end -}}
