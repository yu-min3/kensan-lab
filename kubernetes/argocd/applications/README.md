# Argo CD Applications directory

Argo CD `Application` and `ApplicationSet` CRs live here. `platform-root-app`
scans this directory recursively and deploys everything it finds.

## Which pattern to use

In practice only three things use an ApplicationSet: **observability** and
**vault-{database,transit}-engine**. Everything else — namespaces included — is
an individual Application. See [ADR-003](../../../docs/adr/003-applicationset-migration-strategy.md)
and its Addendum (2026-06-07) for the reasoning.

> ⚠️ **Never write `directory.recurse: false` on its own.** A `directory` block
> that contains nothing but zero values is dropped from the live Application spec
> by the API / Argo CD, which leaves git ≠ live as a permanent OutOfSync
> (demonstrated in PR #382 → #383; keycloak and backstage had carried this drift
> since the kustomize removal in #303/#304 on 2026-05-10).
> Write a `directory` block only when you need `include` / `exclude` — in that
> case keeping `recurse: false` alongside them is fine.

| Category | Pattern | Why |
|---------|---------|------|
| **observability/** | ApplicationSet | Uniform structure across Helm multi-source components, parameterised through a Git file generator (`kubernetes/observability/*/config.json`). The reference implementation |
| **secrets/vault-database-engine/**, **vault-transit-engine/** | ApplicationSet (per instance) | A homegrown chart plus per-instance `platform-values/` mass-produced by a Git file generator. The shared part is `app-shared.yaml` (an individual Application) |
| **namespaces/** (formerly environments/) | Individual Application | Namespace-lifecycle apps. `namespace.yaml` sits alongside the component it belongs to and is picked out with `directory.include`. The inconsistent `-namespace` suffix on app names is left in place on purpose, because renaming means prune + create (ADR-003 Addendum). There is no `config.json` |
| **network/** | Individual Application | sync-wave dependencies and `ignoreDifferences` are specific to each app |
| **auth/** | Individual Application | Keycloak, oauth2-proxy, and vault-oidc-auth are each shaped differently |
| **secrets/** (the rest) | Individual Application | sync-waves apply. Vault, external-secrets, cert-manager, and so on |
| **policy/** | Individual Application | Kyverno itself plus kyverno-policies |
| **storage/** | Individual Application | Longhorn carries many specific settings, `Prune=false` among them |
| **gitops/** | Individual Application | Argo CD manages itself; complex `ignoreDifferences` |
| **backstage/** | Individual Application | A single app |
| **apps/** | Individual Application | One `app.yaml` per app, with image tags updated individually (the in-repo app-kensan is the working example) |

## Adding a new component

### Adding to observability (ApplicationSet)

1. Create `kubernetes/observability/<name>/values.yaml`
2. Create `kubernetes/observability/<name>/config.json` (copy the shape from an existing one)
3. Put any additional manifests in `kubernetes/observability/<name>/resources/`
4. Commit and push — the ApplicationSet generates the Application

### Adding a namespace (individual Application)

There is no ApplicationSet here, so no `config.json` is needed.

1. Put `namespace.yaml` in the component's own directory — do not create a directory just for it
2. Create `kubernetes/argocd/applications/namespaces/<name>/app.yaml` with
   `directory.include: 'namespace.yaml'`, modelled on an existing ns-lifecycle app
3. Commit and push — the root app picks it up on its recursive scan and syncs it
