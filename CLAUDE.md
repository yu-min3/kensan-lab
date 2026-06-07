# CLAUDE.md

## Repository Overview

**kensan-lab** — Kubernetes GitOps platform for bare-metal multi-arch cluster. Owned by Platform Engineers (PE); App Developers (AD) deploy via Backstage.

- **What this is / tech stack / hardware**: [`README.md`](./README.md)
- **Directory layout conventions (Pattern A/B)**: [`kubernetes/README.md`](./kubernetes/README.md)
- **Doc-layout discipline (where to find / write docs)**: [`docs/concepts/doc-layout.md`](./docs/concepts/doc-layout.md) — SoT map + the human/AI two-output model

## Mandatory Constraints

1. **GitOps only**: ALL infrastructure changes via Git → Argo CD. No direct `kubectl apply`（例外: push 前の動作確認の一時適用のみ。→ `.claude/rules/gitops-workflow.md` Verification Exception）.
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
| `/secret-health` | Secret 4 方式（Vault/ESO/SealedSecret）の一括健全性チェック |
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
| `design-system.md` | Whetstone UI tokens/components — 全 app の UI を書く前に読む |

## Domain & Network

- **ドメイン**: `yu-min3.com`（DNS 権威は AWS Route53。Cloudflare は Tunnel での edge 公開のみで DNS 権威ではない）
- **LB IP range**: `192.168.0.240-249`
- **GitHub org**: `yu-min3`
- フォーク時は [`docs/getting-started/configuration.md`](./docs/getting-started/configuration.md) でドメイン等を置換

## User Preferences

- **コマンド出力**: `temp/` に `.sh` + 実行権限で書き出す（詳細: `.claude/rules/gitops-workflow.md` の Script Output Rule が SoT）
- **言語**: 日本語での対話を優先
- **コミット**: Conventional Commits 形式で簡潔に 1 文。1 行目 50 文字以内、本文・trailer 不要（diff を見ればわかる）
  - 例: `feat(policy): Kyverno 導入`
  - 例: `fix(argocd): overlay path bug 修正`
  - 例: `docs(adr): ADR-012 追加`
  - 例: `chore(helm): cilium 1.18.4 へ更新`
  - 例: `refactor(vault): values 整理`
