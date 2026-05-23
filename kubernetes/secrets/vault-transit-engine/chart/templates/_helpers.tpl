{{/*
========================================================================
Smart-default + override helpers
PE 規約 (chart/values.yaml の default) に対し、AD values で override 可能。
========================================================================
*/}}

{{/*
name: Vault role / policy 名 (e.g., transit-kensan-users)
通常は ApplicationSet が helm.parameters で inject するので必須扱い。
values file で override も可能。
*/}}
{{- define "vte.name" -}}
{{- .Values.name | required "name required (set via ApplicationSet helm.parameters or in values file)" -}}
{{- end -}}

{{/*
basename: name から namePrefix を剥がしたもの
e.g., "transit-kensan-users" → "kensan-users"
*/}}
{{- define "vte.basename" -}}
{{- $name := include "vte.name" . -}}
{{- trimPrefix .Values.namePrefix $name -}}
{{- end -}}

{{/*
allKeyNames: keyName + extraKeyNames を 1 list に
policy template で encrypt/decrypt path を render する
*/}}
{{- define "vte.allKeyNames" -}}
{{- $primary := .Values.keyName | required "keyName required" -}}
{{- $all := concat (list $primary) .Values.extraKeyNames -}}
{{- $all | toJson -}}
{{- end -}}

{{/*
vcoAuth: VCO が Vault に login する際の認証 block
本 chart が render する VCO CR (Policy, KubernetesAuthEngineRole) 共通。
*/}}
{{- define "vte.vcoAuth" -}}
path: kubernetes
role: {{ .Values.vcoAuthRole }}
{{- end -}}
