# CLAUDE.md

## Repository Overview

**kensan-lab** — Kubernetes GitOps platform for bare-metal multi-arch cluster. Owned by Platform Engineers (PE); App Developers (AD) deploy via Backstage.

- **What this is / tech stack / hardware**: [`README.md`](./README.md)
- **Directory layout conventions (Pattern A/B)**: [`kubernetes/README.md`](./kubernetes/README.md)
- **Where to find / read docs (agent index)**: [`docs/agents-index.md`](./docs/agents-index.md) — deterministic map into the docs "Operate & Reference" zone
- **Doc-layout discipline (where to write what)**: [`docs/concepts/doc-layout.md`](./docs/concepts/doc-layout.md)

## Mandatory Constraints

1. **GitOps only**: ALL infrastructure changes via Git → Argo CD. No direct `kubectl apply`.
2. **Container runtime**: Default は Docker (`docker buildx` で multi-arch build)。`backstage/app/Makefile` の `CONTAINER_RUNTIME ?= docker` パターンで Podman 切替も可。
3. **No rendered manifests**: Argo CD renders Helm charts natively. Never commit `helm template` output.
4. **Secrets**: dynamic creds via Vault + External Secrets; bootstrap creds via Sealed Secrets. Raw secrets in `temp/` only — commit only sealed/encrypted YAMLs.
5. **No .env commits**: Sensitive tokens stay out of Git.

## Quick Reference

```bash
kubectl get nodes                          # Cluster health
kubectl get applications -n argocd         # GitOps status
kubectl get gateway -A                     # Gateways
kubectl get certificate -A                 # Certificates
kubectl get pods -A --field-selector=status.phase!=Running,status.phase!=Succeeded
```

Backstage: `cd backstage/app && make {install,dev,all TAG=...}`

## Skills (Slash Commands)

| Command | Purpose |
|---------|---------|
| `/sealed-secret <name> <ns>` | Create and seal a new secret |
| `/helm-upgrade <component> <ver>` | Upgrade Helm chart version |
| `/new-component <category> <name>` | Scaffold new infra component |
| `/cluster-status` | Quick cluster health check |
| `/troubleshoot <component>` | Diagnose component issues |
| `/cert-check` | Certificate health & expiry check |
| `/argocd-sync [app]` | Check sync status & drift |

## Domain Rules (`.claude/rules/`)

| Rule File | Scope |
|-----------|-------|
| `gitops-workflow.md` | GitOps principles, deploy order, container runtime |
| `helm-multisource.md` | 3-file pattern details |
| `kubernetes-cluster.md` | Node topology, scheduling, storage |
| `network-ingress.md` | Cilium, Gateways, edge, certs |
| `security-secrets.md` | Vault + ESO, Sealed Secrets, Reloader, cert-manager, GHCR |
| `environment-separation.md` | PE/AD roles, multi-repo, namespaces (ADR-006) |

## Domain & Network

- **ドメイン**: `yu-min3.com` (Cloudflare DNS)
- **LB IP range**: `192.168.0.240-249`
- **GitHub org**: `yu-min3`
- フォーク時は [`docs/getting-started/configuration.md`](./docs/getting-started/configuration.md) でドメイン等を置換

## User Preferences

- **コマンド出力**: シェルコマンドを提示する際は `temp/` に `.sh` で書き出す（改行崩れ防止）
- **言語**: 日本語での対話を優先
- **コミット**: 1 行目 50 文字以内、本文不要（diff を見ればわかる）
