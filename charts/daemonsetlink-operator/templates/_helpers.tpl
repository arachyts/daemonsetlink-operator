{{/*
Expand the name of the chart.
*/}}
{{- define "daemonsetlink-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "daemonsetlink-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels for all resources created by this chart.
*/}}
{{- define "daemonsetlink-operator.labels" -}}
helm.sh/chart: {{ include "daemonsetlink-operator.chart" . }}
app.kubernetes.io/name: {{ include "daemonsetlink-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/component: controller-manager
{{- end -}}

{{/*
Labels for the Pod template within the Deployment.
*/}}
{{- define "daemonsetlink-operator.podLabels" -}}
control-plane: controller-manager
app.kubernetes.io/name: {{ include "daemonsetlink-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: controller-manager
{{- with .Values.podLabels }}
  {{- toYaml . | nindent 0 }}
{{- end }}
{{- end -}}

{{/*
Service Account name.
*/}}
{{- define "daemonsetlink-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
  {{- if .Values.serviceAccount.name -}}
    {{- .Values.serviceAccount.name | trunc 63 | trimSuffix "-" -}}
  {{- else -}}
    {{- printf "%s-controller-manager" (include "daemonsetlink-operator.name" .) | trunc 63 | trimSuffix "-" -}}
  {{- end -}}
{{- else -}}
  {{- required "A ServiceAccount name is required if serviceAccount.create is false (.Values.serviceAccount.name)" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Controller Manager Deployment name.
Example: {{ .Release.Name }}-dslink-operator-controller-manager
*/}}
{{- define "daemonsetlink-operator.controllerManagerName" -}}
{{- printf "%s-controller-manager" (include "daemonsetlink-operator.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
RBAC Resource Names
*/}}
{{- define "daemonsetlink-operator.leaderElectionRoleName" -}}
{{- printf "%s-leader-election-role" (include "daemonsetlink-operator.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "daemonsetlink-operator.managerRoleName" -}}
{{- printf "%s-manager-role" (include "daemonsetlink-operator.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "daemonsetlink-operator.daemonsetlinkEditorRoleName" -}}
{{- printf "%s-daemonsetlink-editor-role" (include "daemonsetlink-operator.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "daemonsetlink-operator.daemonsetlinkViewerRoleName" -}}
{{- printf "%s-daemonsetlink-viewer-role" (include "daemonsetlink-operator.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Effective namespace for namespaced resources.
Defaults to .Release.Namespace if .Values.namespace is not set.
*/}}
{{- define "daemonsetlink-operator.namespace" -}}
{{- .Values.namespace | default .Release.Namespace -}}
{{- end -}}

{{/*
Image tag to use. Defaults to chart's appVersion if not specified in .Values.image.tag
*/}}
{{- define "daemonsetlink-operator.imageTag" -}}
{{- .Values.image.tag | default .Chart.AppVersion -}}
{{- end -}}