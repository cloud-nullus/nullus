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

{{- define "nullus.api.selectorLabels" -}}
{{ include "nullus.selectorLabels" . }}
app.kubernetes.io/component: api
{{- end -}}

{{- define "nullus.web.selectorLabels" -}}
{{ include "nullus.selectorLabels" . }}
app.kubernetes.io/component: web
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

{{- define "nullus.database.host" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "%s-postgresql" (include "nullus.fullname" .) }}
{{- else }}
{{- .Values.externalDatabase.host }}
{{- end }}
{{- end -}}

{{- define "nullus.database.port" -}}
{{- if .Values.postgresql.enabled -}}
5432
{{- else -}}
{{ .Values.externalDatabase.port }}
{{- end -}}
{{- end -}}

{{- define "nullus.database.name" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.database }}
{{- else }}
{{- .Values.externalDatabase.name }}
{{- end }}
{{- end -}}

{{- /*
번들 PostgreSQL 을 쓸 때 API 가 참조하는 비밀번호는 서브차트가 실제로 설정한
postgresql.auth.password 여야 한다. secrets.dbPassword 를 그대로 쓰면 두 값이
어긋났을 때 API 가 DB 인증에 실패한다(기본값이 change-me / nullus 로 불일치).
host/port/name/username 과 같은 규칙으로 한쪽에서만 결정한다.
*/ -}}
{{- define "nullus.database.password" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.password }}
{{- else }}
{{- .Values.secrets.dbPassword }}
{{- end }}
{{- end -}}

{{- define "nullus.database.username" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.username }}
{{- else }}
{{- .Values.externalDatabase.username }}
{{- end }}
{{- end -}}
