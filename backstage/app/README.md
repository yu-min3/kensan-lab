# Backstage Developer Portal

Production Backstage app for the kensan-lab GitOps platform. Application scaffolding templates live in `templates/` (integrated, not a separate repo). Bundled Yarn 4 in `.yarn/releases/` — no global Yarn install needed.

## Quick Reference

```bash
make help      # show all targets and required env vars
make install   # install dependencies (bundled Yarn 4)
make dev       # local dev server on :7007
make build     # build container image
make push      # build + push to GHCR
make all TAG=v0.0.7   # build + push + bump kustomization.yaml
```

`make` is the single entry point for build/deploy. The full target list, container runtime switch, and required env vars are documented in `make help`.

## Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Node.js | 22.x | Use nvm for version management |
| Docker | latest | `CONTAINER_RUNTIME=podman` to switch |
| GitHub PAT | `packages:write` | GHCR push + GitHub integration |
| Memory | 2 GB+ free | For dependency installation (`NODE_OPTIONS=--max-old-space-size=4096` if low) |

> **Yarn is NOT required globally** — Yarn 4 is bundled in `.yarn/releases/`.

## Configuration

```
backstage/app/
├── app-config.yaml              # base
├── app-config.kubernetes.yaml   # k8s overlay
└── app-config.local.yaml        # local overrides (env-var-driven)
```

All three are committed. Sensitive values come from env vars. Create `../.env` (one level up, repository root) with:

```bash
GITHUB_USER=your-username
GITHUB_GHCR_PAT=ghp_xxxxxxxxxxxxx
```

`app-config.local.yaml` references `${GITHUB_GHCR_PAT}` for GitHub integration.

## Deployment

GitOps via Argo CD — `make all TAG=v0.0.7` builds, pushes to GHCR, and updates `kubernetes/backstage/overlays/prod/kustomization.yaml`. Commit + push the kustomization change to trigger sync.

Manual `kubectl apply` is **not** recommended — auto-sync + self-heal will revert manual changes. To pause for emergencies: `argocd app set backstage --sync-policy none`.

## Architecture

### Directory Layout

```
backstage/app/
├── .yarn/releases/yarn-4.4.1.cjs    # bundled Yarn 4
├── packages/{app,backend}/           # frontend / backend
├── plugins/                          # custom Backstage plugins
├── templates/                        # scaffolder templates (bundled into image)
├── catalog/                          # production catalog data
├── app-config*.yaml                  # 3-file config (see above)
├── Makefile                          # entry point — see `make help`
└── yarn.sh                           # wrapper for bundled Yarn 4
```

### Design Notes

- **Templates are co-located**, not in a separate repo — simpler version control, image bundles them in the production layer.
- **Database**: in-memory SQLite locally, PostgreSQL in production (switched via env vars).
- **Secrets**: env vars locally, Sealed Secrets in cluster.

## Application Scaffolding

Backstage scaffolds new apps from `templates/`. AD selects a template in the UI → Backstage generates a new repo with manifests/ + Dockerfile, and auto-commits an Argo CD Application CR into kensan-lab.

### Template Structure

```
templates/fastapi-template/
├── template.yaml          # scaffolder definition
├── catalog-info.yaml      # catalog registration
└── skeleton/              # files written into the generated app repo
    ├── app/main.py
    ├── manifests/         # flat manifests (deployment, service, httproute, …)
    ├── Dockerfile
    └── docs/              # TechDocs
```

> **Note**: Currently flat manifests (Argo CD directory source). Kustomize overlays for dev/prod were retired after the dev/prod split was dropped — see [`.claude/rules/environment-separation.md`](https://github.com/yu-min3/kensan-lab/blob/main/.claude/rules/environment-separation.md).

### AD Workflow

1. Develop code in `app/`.
2. Build + push image with versioned tag (via the generated app's `make build TAG=...`).
3. Bump image tag in `manifests/deployment.yaml`.
4. Commit + push → Argo CD syncs.
5. Update TechDocs in `docs/`.

### Key Properties

- **Self-service**: AD owns app lifecycle, no PE involvement after scaffold.
- **Security**: AD cannot modify infra and has no access to other apps' namespaces.
- **Observability**: ServiceMonitor enables Prometheus scraping automatically.

## Troubleshooting

### Memory during install

```bash
export NODE_OPTIONS="--max-old-space-size=4096"
make install
```

### Yarn command not found

Use the bundled Yarn: `./yarn.sh install` or `node .yarn/releases/yarn-4.4.1.cjs install`.

### Template not loading

Check `app-config.yaml`:

```yaml
catalog:
  locations:
    - type: file
      target: ../../templates/fastapi-template/template.yaml
      rules:
        - allow: [Template]
```

### Database connection error in cluster

```bash
kubectl exec -n backstage deployment/backstage -- env | grep POSTGRES
kubectl get pods -n backstage
kubectl logs -n backstage statefulset/postgresql
```

### Image pull error in cluster

```bash
kubectl get secret backstage-secret -n backstage -o yaml
kubectl get secret backstage-secret -n backstage -o jsonpath='{.data.GITHUB_TOKEN}' | base64 -d
```

## Testing

```bash
make test        # unit tests (yarn test:all)
make test-e2e    # Playwright E2E
```

E2E test report: `npx playwright show-report e2e-test-report`.

## Resources

- Backstage docs: https://backstage.io/docs
- Yarn 4 docs: https://yarnpkg.com
- Platform repo: https://github.com/yu-min3/kensan-lab

## Version

- Backstage 1.44.0 (see `backstage.json`)
- Node.js 22.x
- Yarn 4.4.1 (bundled)

## License

Apache 2.0.
