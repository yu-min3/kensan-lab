# Agent Index

**For AI agents and operators.** This is the deterministic entry point into the kensan-lab docs: it maps *what you need* to *the exact file that owns it*. If you are a human reading to **understand** the platform, use the **📖 Understand** section of the nav instead — diagrams and narrative live there. This **🤖 Operate & Reference** zone is optimized for certainty and coverage, not storytelling.

!!! abstract "Two front doors, one set of facts"
    The **Understand** zone explains *why / how* with diagrams and narrative. This **Operate & Reference** zone is *where you execute and look things up*. Every fact lives in exactly **one** file (single source of truth) — this index points you to it, it does not restate it. If you find the same fact in two places, that is a bug: collapse it to the SoT and link.

## Start here

| You need to… | Go to |
|---|---|
| Find which file **owns** a fact (source of truth) | [Doc layout — SoT map](concepts/doc-layout.md) |
| Decide **where to write** a new fact | [Doc layout — decision flow](concepts/doc-layout.md) |
| Know the rule that fires while **editing** a path | [`.claude/rules/`](https://github.com/yu-min3/kensan-lab/tree/main/.claude/rules) — glob-triggered, thin summaries linking back here |
| Read mandatory constraints + available skills | [`CLAUDE.md`](https://github.com/yu-min3/kensan-lab/blob/main/CLAUDE.md) |
| Understand directory layout (Pattern A/B) | [`kubernetes/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/kubernetes/README.md) |

## Operate & Reference inventory

Dense, procedural, look-up material. Each row says *when to reach for it*.

### Bootstrapping — one-time cluster bring-up

| Doc | Reach for it when… |
|---|---|
| [Overview](bootstrapping/index.md) | You need the bootstrap order and the Argo CD multi-source pattern used at bring-up |
| [Stage 1 — Vault HA](bootstrapping/12-vault-stage1.md) | Initializing / unsealing Vault for the first time (AWS IAM auto-unseal, root token, KV mount) |
| [Grafana independent setup](bootstrapping/11-grafana-independent.md) | Standing Grafana up independently of the rest of the observability stack |
| [Add worker node (M4 Neo)](bootstrapping/add-worker-node-m4neo.md) | Joining a new worker to the cluster |

### Secret management

| Doc | Reach for it when… |
|---|---|
| [Secret Management](secret-management/index.md) | You need the 4-method matrix (Vault dynamic / static via ESO / Transit / Sealed Secrets), full secret inventory, VCO coverage, or which method to pick. **SoT for secrets.** |

### Auth integration

| Doc | Reach for it when… |
|---|---|
| [ArgoCD ↔ Keycloak (Path B)](auth/argocd-keycloak-integration.md) | Wiring Argo CD's OIDC client to Keycloak (ExternalSecret + ConfigMap) |
| [Gateway OIDC (Path A)](auth/gateway-oidc.md) | Operating OIDC at the Istio Gateway via oauth2-proxy ext_authz |
| [oauth2-proxy](auth/oauth2-proxy.md) | Configuring the oauth2-proxy itself (sidecar, scheduling, cookie/redirect) |

### Runbooks — recurring operational procedures

| Doc | Reach for it when… |
|---|---|
| [Vault Raft auto-join](runbooks/vault-raft-join.md) | A Vault pod won't rejoin the Raft quorum (retry_join) |
| [Cilium WiFi stability](runbooks/cilium-wifi-stability.md) | Diagnosing Cilium L2 / lease instability (legacy WiFi-era tuning) |
| [Cilium update strategy](runbooks/cilium-update-strategy.md) | Upgrading the Cilium DaemonSet under the `maxUnavailable=1` constraint |
| [Longhorn restore test](runbooks/longhorn-restore-test.md) | Validating Longhorn backup / restore (disaster recovery) |
| [ArgoCD repo-server tuning](runbooks/argocd-repo-server-tuning.md) | repo-server probe timeouts / performance under load |

### Guides

| Doc | Reach for it when… |
|---|---|
| [Raspberry Pi WiFi stabilization](guides/wifi-stabilization.md) | Tuning Pi WiFi settings (legacy WiFi-only era reference) |

### Operations — environment cleanup

| Doc | Reach for it when… |
|---|---|
| [kensan dev cleanup](operations/kensan-dev-cleanup.md) | Tearing down the kensan dev environment |
| [Keycloak dev cleanup](operations/keycloak-dev-cleanup.md) | Tearing down Keycloak dev artifacts |
| [Vault role & gateway cleanup](operations/vault-dev-role-and-gateway-cleanup.md) | Removing dev Vault roles + the dev gateway |

### Incidents — postmortems

| Doc | Reach for it when… |
|---|---|
| [2026-03-02 system-infra sync blocked](incidents/2026-03-02-system-infra-sync-blocked.md) | Understanding a past Argo CD auto-sync block and how it was resolved |

## Understand zone — concepts an agent may still need

These are human-narrative pages; read them for context, not as command references.

- **Architecture overviews** — [infrastructure](architecture/infrastructure.md), [auth](architecture/auth.md), [secrets](architecture/secrets.md), [network](architecture/network.md), [observability](architecture/observability.md)
- **Decisions (the why)** — [ADR index](adr/index.md) → individual ADRs
- **Design concepts** — [Namespace labels](concepts/namespace-label-design.md), [NetworkPolicy](concepts/network-policy-guide.md), [Doc layout](concepts/doc-layout.md)
