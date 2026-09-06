# Argo CD Application manifests (apps category)

Argo CD `Application` CRs for the **applications** running on the platform. Each
app has its own subdirectory, and the `app.yaml` inside it defines a single
`Application`.

## Directory structure

```
apps/
├── app-kensan/
│   └── app.yaml    # the current kensan (in-repo chart + per-app ns, 3 sources)
└── README.md
```

> ⚠️ The filename is `app.yaml`. Older documents referring to `argocd-apps.yaml`
> do not match what is here.

## The pattern in use: an in-repo app (app-kensan)

The current app has no external repository; it is self-contained in this
monorepo. `app-kensan/app.yaml` references the PE-provided generic chart
`charts/app-base` as a **multi-source** Application with three sources:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app-kensan
  namespace: argocd
  annotations:
    argocd.argoproj.io/sync-options: Prune=false   # protects the Application CR from prune
spec:
  project: app-project
  sources:
    # 1. the generic chart owned by PE
    - repoURL: https://github.com/yu-min3/kensan-lab
      targetRevision: main
      path: charts/app-base
      helm:
        releaseName: app-kensan
        valueFiles:
          - $values/kubernetes/apps/app-kensan/values.yaml
    # 2. a ref source so the values file can be addressed
    - repoURL: https://github.com/yu-min3/kensan-lab
      targetRevision: main
      ref: values
    # 3. raw manifests (namespace, PVC, syncthing, ...)
    - repoURL: https://github.com/yu-min3/kensan-lab
      targetRevision: main
      path: kubernetes/apps/app-kensan/resources
      directory:
        recurse: true
  destination:
    server: https://kubernetes.default.svc
    namespace: app-kensan           # per-app namespace (ADR-006)
  syncPolicy:
    automated: { prune: true, selfHeal: true }
    syncOptions:
      - CreateNamespace=false       # managed by namespace.yaml, which carries the labels
      - ServerSideApply=true
```

How to use `charts/app-base` is documented in
[charts/app-base/README.md](../../../../charts/app-base/README.md). App-specific
values go in `kubernetes/apps/app-<name>/values.yaml`.

The old kensan's `kensan/app.yaml` was removed in the Phase 7 cutover (PR #394).
Its source is archived at the tag `kensan-legacy-final` (ADR-017).

## The planned flow: Backstage-scaffolded apps

In the future flow, external app repositories are mass-produced from a Backstage
Software Template. The template creates the app repository (named
`kensan-lab-apps-<name>`) and a pull request against this repository containing
the `Application` CR; a platform engineer reviews and merges it, and Argo CD
syncs automatically. That automation is still being built — the only pattern
running today is the in-repo one above.
