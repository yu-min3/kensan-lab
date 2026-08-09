{{/*
Sync policy shared by every explore Application.

It is identical to the bare-metal one except for the finalizer: explore
Applications carry none, because the cluster is thrown away wholesale by
`make explore-down` and a resources-finalizer would only stall that deletion.
*/}}
{{- define "explore.syncPolicy" -}}
syncPolicy:
  automated:
    prune: true
    selfHeal: true
  syncOptions:
    - CreateNamespace=false
    - ServerSideApply=true
  retry:
    limit: 5
    backoff:
      duration: 5s
      factor: 2
      maxDuration: 3m
{{- end }}

{{/*
The in-cluster destination. Explore is always single-cluster.
*/}}
{{- define "explore.destination" -}}
destination:
  server: https://kubernetes.default.svc
  namespace: {{ .namespace }}
{{- end }}
