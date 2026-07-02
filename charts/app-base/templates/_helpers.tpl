{{- define "app-base.name" -}}
{{- .Values.nameOverride | default .Release.Name -}}
{{- end }}

{{- define "app-base.labels" -}}
app.kubernetes.io/name: {{ include "app-base.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "app-base.selectorLabels" -}}
app.kubernetes.io/name: {{ include "app-base.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "app-base.pvcName" -}}
{{- .Values.pvc.name | default (printf "%s-data" (include "app-base.name" .)) -}}
{{- end }}
