# Golden Path: shipping an app via Backstage

The end-to-end walkthrough of the platform's fully-automated App Developer (AD) path: **one Backstage form → a running, observable, SSO-protected application** — without the developer ever editing platform YAML.

This is the human-facing counterpart to the architecture page ([`kubernetes/backstage/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/kubernetes/backstage/README.md)) and the role model ([PE / AD split](https://github.com/yu-min3/kensan-lab/blob/main/.claude/rules/environment-separation.md)).

<!-- TODO(screenshots): template form / scaffolder run / catalog page — capture from the live Backstage once convenient -->

## What one form submission produces

| Artifact | Where | Owner |
|---|---|---|
| App repository (code, Dockerfile, CI, `manifests/`, TechDocs) | `github.com/<owner>/<repo>` (new) | AD |
| Argo CD `Application` CR | PR against `kensan-lab` → `kubernetes/argocd/applications/apps/<name>/` | PE reviews, then Argo CD |
| Catalog entry + TechDocs | Backstage (`catalog-info.yaml` auto-registered) | AD |

## Walkthrough

### 1. Create from the template

Open Backstage (`backstage.platform.yu-min3.com` — SSO happens at the Gateway, no separate Backstage login) → **Create…** → **FastAPI Application**.

Fill in:

- **Application Name** — lowercase / digits / hyphens. Becomes the Argo CD Application name (`app-<name>`), the catalog component name, and the image name.
- **Owner** — picked from the catalog (Group/User).
- **Repository Location** — where the new app repo is created. Argo CD authenticates to any repo under the org prefix via credential templates, so no per-repo secret setup is needed.
- **Domain / System** — catalog placement ([`backstage/catalog/`](https://github.com/yu-min3/kensan-lab/tree/main/backstage/catalog) defines the domains).

### 2. The scaffolder runs three actions

1. **`publish:github`** — creates the app repository from the skeleton: FastAPI service, Dockerfile, a GitHub Actions build-and-push workflow (GHCR), flat `manifests/` (Deployment / Service / HTTPRoute / AuthorizationPolicy / ServiceMonitor), `catalog-info.yaml`, and an MkDocs TechDocs skeleton.
2. **`publish:github:pull-request`** — opens a PR against `kensan-lab` adding the Application CR under `kubernetes/argocd/applications/apps/<name>/`. Deliberately a PR, not a direct commit: the platform keeps its review gate even on the automated path.
3. **`catalog:register`** — the new component appears in the service catalog with TechDocs.

### 3. Platform Engineer merges the PR

Review checklist is short by construction — the CR is generated, scoped to `app-project`, and can only deploy into AD-owned namespaces. On merge, `platform-root` (App-of-Apps) discovers the new Application automatically; no Argo CD configuration is touched.

### 4. The app goes live

Argo CD syncs the app repo's `manifests/` into the app namespace. The generated manifests already integrate the platform:

- **Ingress + SSO** — HTTPRoute attaches to the shared Gateway; OIDC is enforced at the edge (oauth2-proxy ext_authz), so the app trusts authenticated headers and implements no login.
- **Zero-trust defaults** — the namespace baseline (default-deny NetworkPolicy, PSA, Istio mTLS) applies without app-side configuration; the skeleton ships the explicit `AuthorizationPolicy` it needs.
- **Observability** — a ServiceMonitor is included; telemetry goes to the central OTel Collector ([integration guide](observability-integration.md)).

### 5. Iterate

Day-2 development never re-enters the platform repo: push code → CI builds a multi-arch image to GHCR → bump the image tag in the app repo's `manifests/` → Argo CD syncs. Secrets, when needed, are requested from the platform rails ([secret decision framework](../secret-management/index.md)).

## Current state and target model

The namespace model is one namespace per app (`app-<name>`, [ADR-006](../adr/006-namespace-naming.md) / [ADR-014](../adr/014-namespace-naming-label-contract-v2.md)) with the namespace and HTTPRoute owned by the app side — `app-kensan` runs this pattern, and the shared `app-prod` landing zone was retired with the migration (2026-07). The scaffolder template still generates a CR targeting the retired `app-prod` namespace and legacy dev/prod wording; updating it to the per-app model is a known follow-up, so treat the template's namespace output as pending that update.
