---
type: note
title: "Private canary operations"
tags: [sense, canary]
status: active
created: 2026-10-04
updated: 2026-10-04
---

# Private canary

Offline-generated FastAPI source for the existing kensan-lab repository. This generation does not establish an authenticated Backstage OIDC run.

The image owns `/api/release`; `/api/marker` stores a bounded marker in the retained canary-data PVC. Local tests use a temporary data directory.

Run `make test` or `make build` locally. Initial private image publication needs separate review of the exact source SHA, tag and new-package approval. Later images use the independently gated image workflow, and GitOps values pin its verified digest. Generation performs no publication or deployment.

## Fixed deployment and credential references

| Item | Fixed reference |
|---|---|
| Namespace / Argo Application | app-canary / app-canary |
| Service / ServiceAccount | canary / canary |
| Container / Service port | 8000 / 8000 |
| Internal user path | /api/release (initial v1, reviewed source change v2) |
| Image package | ghcr.io/yu-min3/kensan-lab/canary |
| Namespace-local image pull Secret | app-canary/ghcr-pull-secret |
| ExternalSecret store | ClusterSecretStore vault-backend |
| Existing Vault reference | ghcr/pull-token, property dockerconfigjson |
| PVC | app-canary/canary-data (Longhorn, 1Gi, Prune=false) |

The existing app-base chart creates the namespace-local ExternalSecret and binds the resulting pull Secret to the canary ServiceAccount. Review the existing Vault reference and the token's access to the private canary package before deployment. Do not copy a Secret from another namespace, print credential contents, or create new Vault privileges as part of generation.

## Initial private-package review

1. Check package existence and visibility with an owner credential that has `read:packages`. A 404 from a credential without that scope is inconclusive.
2. Separately review the exact source commit SHA, initial immutable tag v1, private package creation, and rollback. No initial publication is authorized by generation.
3. The Canary CI in this source change builds and tests only. It has no image-publishing job or package token. A separate, reviewed workflow and credential setup are required before the first private-package publication; the source/CI change grants no publication authority.
4. Require the post-push private visibility/repository/digest record and a namespace-local pull Secret ready from the existing Vault reference before adopting the image or deploying.

Health and release checks use the selected Pod on port 8000 through the read-only Kubernetes pods/proxy API. The direct Pod network is not assumed reachable from the host. If authorized observation fails, deployment acceptance stays blocked. Do not enable HTTPRoute, NodePort, Gateway, auth routes, or a public hostname to bypass that prerequisite.
