{{/*
Expand the chart name.
*/}}
{{- define "ldapapi-ng.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Fully qualified app name. Truncated to 63 chars (DNS-1123 label limit).
If release name contains chart name it is used as-is.
*/}}
{{- define "ldapapi-ng.fullname" -}}
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
Chart label (chart name + version, sanitised).
*/}}
{{- define "ldapapi-ng.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every object.
*/}}
{{- define "ldapapi-ng.labels" -}}
helm.sh/chart: {{ include "ldapapi-ng.chart" . }}
{{ include "ldapapi-ng.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: ldapapi-ng
{{- end }}

{{/*
Selector labels — must stay stable across upgrades.
*/}}
{{- define "ldapapi-ng.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ldapapi-ng.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "ldapapi-ng.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ldapapi-ng.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image reference (repository + tag).
*/}}
{{- define "ldapapi-ng.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end }}

{{/*
Absolute path to the mounted LDAP CA certificate (if enabled).
*/}}
{{- define "ldapapi-ng.caCertPath" -}}
{{- printf "%s/%s" .Values.ldap.caCert.mountPath .Values.ldap.caCert.fileName -}}
{{- end }}

{{/*
Validate the combination of auth.mode and exposure flags.
Fails rendering if a gateway-mode release also enables Ingress/HTTPRoute,
or if no LDAP host/base DN is configured.
*/}}
{{- define "ldapapi-ng.validate" -}}
{{- if not .Values.ldap.host -}}
{{- fail "ldap.host is required — set it in values.yaml" -}}
{{- end -}}
{{- if not .Values.ldap.baseDN -}}
{{- fail "ldap.baseDN is required — set it in values.yaml" -}}
{{- end -}}
{{- if not (or (eq .Values.auth.mode "gateway") (eq .Values.auth.mode "standalone")) -}}
{{- fail (printf "auth.mode must be 'gateway' or 'standalone', got %q" .Values.auth.mode) -}}
{{- end -}}
{{- if eq .Values.auth.mode "gateway" -}}
  {{- if .Values.ingress.enabled -}}
  {{- fail "ingress.enabled must be false in gateway mode — the API must only be reachable via the gateway" -}}
  {{- end -}}
  {{- if .Values.httpRoute.enabled -}}
  {{- fail "httpRoute.enabled must be false in gateway mode — the API must only be reachable via the gateway" -}}
  {{- end -}}
{{- end -}}
{{- if and .Values.ldap.caCert.enabled .Values.ldap.caCert.content .Values.ldap.caCert.existingConfigMap -}}
{{- fail "ldap.caCert.content and ldap.caCert.existingConfigMap are mutually exclusive" -}}
{{- end -}}
{{- end }}
