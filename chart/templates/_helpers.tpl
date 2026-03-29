{{/*
Expand the name of the chart.
*/}}
{{- define "harvester-vm-dhcp-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "harvester-vm-dhcp-controller.fullname" -}}
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
Return the agent service account name
*/}}
{{- define "harvester-vm-dhcp-controller.agentServiceAccountName" -}}
{{- if .Values.agent.serviceAccount.create }}
{{- default (printf "%s-agent" (include "harvester-vm-dhcp-controller.fullname" .)) .Values.agent.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.agent.serviceAccount.name }}
{{- end }}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "harvester-vm-dhcp-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "harvester-vm-dhcp-controller.labels" -}}
helm.sh/chart: {{ include "harvester-vm-dhcp-controller.chart" . }}
{{ include "harvester-vm-dhcp-controller.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: controller
{{- end }}

{{- define "harvester-vm-dhcp-webhook.labels" -}}
helm.sh/chart: {{ include "harvester-vm-dhcp-controller.chart" . }}
{{ include "harvester-vm-dhcp-webhook.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: webhook
{{- end }}

{{/*
Selector labels
*/}}
{{- define "harvester-vm-dhcp-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "harvester-vm-dhcp-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "harvester-vm-dhcp-webhook.selectorLabels" -}}
app.kubernetes.io/name: {{ include "harvester-vm-dhcp-controller.name" . }}-webhook
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "harvester-vm-dhcp-controller.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "harvester-vm-dhcp-controller.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Return the appropriate apiVersion for rbac.
*/}}
{{- define "harvester-vm-dhcp-controller.rbac.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "rbac.authorization.k8s.io/v1" }}
{{- print "rbac.authorization.k8s.io/v1" }}
{{- else }}
{{- print "v1" }}
{{- end }}
{{- end -}}
