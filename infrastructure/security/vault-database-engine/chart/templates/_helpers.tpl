{{/*
========================================================================
Smart-default + override helpers
PE 規約 (chart/values.yaml の default) に対し、AD values で override 可能。
========================================================================
*/}}

{{/*
name: Vault role 名 (e.g., postgres-backstage)
通常は ApplicationSet が helm.parameters で inject するので必須扱い。
values file で override も可能。
*/}}
{{- define "vdbe.name" -}}
{{- .Values.name | required "name required (set via ApplicationSet helm.parameters or in values file)" -}}
{{- end -}}

{{/*
basename: name から namePrefix を剥がしたもの
e.g., "postgres-backstage" → "backstage"
rootOwner / dbName のデフォルト元に使う
*/}}
{{- define "vdbe.basename" -}}
{{- $name := include "vdbe.name" . -}}
{{- trimPrefix .Values.namePrefix $name -}}
{{- end -}}

{{/*
rootOwner: REASSIGN OWNED BY 先の static user
default = basename (Bitnami auth.username が basename と一致する想定)
*/}}
{{- define "vdbe.rootOwner" -}}
{{- .Values.rootOwner | default (include "vdbe.basename" .) -}}
{{- end -}}

{{/*
dbName: 接続先 DB 名 (Bitnami auth.database default = auth.username)
default = rootOwner
*/}}
{{- define "vdbe.dbName" -}}
{{- .Values.dbName | default (include "vdbe.rootOwner" .) -}}
{{- end -}}

{{/*
host: Postgres FQDN
default = <releaseName>.<ns>.svc.cluster.local
ns 必須 (host 直接指定しない場合)
*/}}
{{- define "vdbe.host" -}}
{{- if .Values.host -}}
{{- .Values.host -}}
{{- else -}}
{{- $ns := .Values.ns | required "ns required (or specify host directly)" -}}
{{- printf "%s.%s.svc.cluster.local" .Values.releaseName $ns -}}
{{- end -}}
{{- end -}}

{{/*
kvAdminPath: Vault KV の admin cred path
完全 convention 化: <kvAdminPathPrefix>/<name>
keys は username / password 固定
*/}}
{{- define "vdbe.kvAdminPath" -}}
{{- printf "%s/%s" .Values.kvAdminPathPrefix (include "vdbe.name" .) -}}
{{- end -}}

{{/*
targetSecretName: app ns に生成される K8s Secret 名
default = "<name>-cred" (e.g., postgres-backstage-cred)
旧 static Secret 名と揃えたい場合は AD values で override
*/}}
{{- define "vdbe.targetSecretName" -}}
{{- .Values.targetSecretName | default (printf "%s-cred" (include "vdbe.name" .)) -}}
{{- end -}}

{{/*
saName: ESO が Vault auth に使う SA 名 (= Vault kubernetes auth role 名)
完全 convention 化: vault-db-<basename>
app ns 側 SA と vault ns 側 KubernetesAuthEngineRole の両方で同名を使う
*/}}
{{- define "vdbe.saName" -}}
{{- printf "vault-db-%s" (include "vdbe.basename" .) -}}
{{- end -}}
