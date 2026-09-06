# apps

Application workloads that the platform exists to run. Infrastructure and
platform services live in the sibling directories; everything here is a tenant.

## Layout

- `app-kensan/` — the kensan knowledge app (`app-kensan` namespace): `values.yaml`
  for the shared `app-base` chart, plus `resources/` for the workspace PVC,
  Syncthing, and its NetworkPolicy
- `app-konro/` — the konro app (`app-konro` namespace)

Each application follows the `app-{name}` flat namespace convention with the
three-axis labels; the Argo CD `Application` CRs that deploy them are in
`../argocd/applications/apps/`.

## Related

- Namespace naming and labels: [ADR-006](../../docs/adr/006-namespace-naming.md) · [ADR-014](../../docs/adr/014-namespace-naming-label-contract-v2.md)
- How an application gets here: [Backstage Golden Path](../../docs/architecture/backstage-golden-path.md)
