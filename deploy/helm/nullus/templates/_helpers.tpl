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

{{/*
OSS SSO 프로비저닝(provisioning_sso)이 쓸 플랫폼 Keycloak 의 관리자 주소.

브라우저가 아니라 API 파드가 호출하는 주소라서 공개 도메인일 필요가 없다 —
web.auth.oidcAuthority(브라우저용 공개 주소)와 혼동하지 말 것.
명시값이 없고 Keycloak 서브차트가 켜져 있으면 클러스터 내부 서비스로 자동 연결한다.
빈 문자열이면 API 가 프로비저닝을 건너뛴다(BYO IdP / SSO 미사용).
*/}}
{{- define "nullus.keycloak.adminUrl" -}}
{{- if .Values.config.keycloak.adminUrl -}}
{{- .Values.config.keycloak.adminUrl -}}
{{- else if .Values.keycloak.enabled -}}
{{- printf "http://%s" (include "common.names.fullname" .Subcharts.keycloak) -}}
{{- end -}}
{{- end -}}

{{/*
관리자 비밀번호가 든 시크릿 이름/키.

서브차트를 쓰면 비밀번호는 Keycloak 차트가 만들거나 랜덤 생성해 자기 시크릿에
넣는다. 그 값을 우리 시크릿으로 복사하면 두 벌이 되어 회전 때 어긋나므로,
원본 시크릿을 그대로 참조한다. 외부 Keycloak(BYO)일 때만 우리 시크릿을 쓴다.
*/}}
{{- define "nullus.keycloak.adminPasswordSecretName" -}}
{{- if .Values.config.keycloak.adminPassword -}}
{{- include "nullus.fullname" . -}}
{{- else if .Values.keycloak.enabled -}}
{{- include "keycloak.secretName" .Subcharts.keycloak -}}
{{- end -}}
{{- end -}}

{{- define "nullus.keycloak.adminPasswordSecretKey" -}}
{{- if .Values.config.keycloak.adminPassword -}}
{{- print "keycloak-admin-password" -}}
{{- else if .Values.keycloak.enabled -}}
{{- include "keycloak.secretKey" .Subcharts.keycloak -}}
{{- end -}}
{{- end -}}
