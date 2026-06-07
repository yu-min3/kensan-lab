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
| `/argocd-sync [app]` | Check sync status & drift |
| `/codex <依頼>` | OpenAI Codex へ委譲（レビュー・セカンドオピニオン・別解生成） |

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
| `collaboration.md` | PR 運用（独断マージ禁止・本文規約）、設計/状況報告は HTML 図示、script 出力 |

## Review Guidelines（エージェントレビュー観点）

レビューエージェント（Claude `/code-review`、Codex `codex exec review` — 後者は `AGENTS.md` symlink 経由で本ファイルを読む）は以下の優先度で指摘する:

- **P0 (block)**: 生 secret の commit（`temp/*-raw.yaml`・`.env`・token / credential 平文）/ rendered Helm manifest（`helm template` 出力）の commit / GitOps バイパス（`kubectl apply` 前提の変更）
- **P1 (warn)**: single-arch image 指定（multi-arch manifest list 必須）/ chart version を Application CR の `targetRevision` 以外で管理 / 新規 PVC での `local-path` 指定（default は `longhorn`）/ 保護が必要なリソースへの `Prune=false` annotation 漏れ
- **P2 (info)**: doc-layout 規約違反 / namespace 命名（`app-{name}`）違反 / HTTPRoute の `parentRefs` と Gateway の不整合

## Domain & Network

- **ドメイン**: `yu-min3.com`（DNS 権威は AWS Route53。Cloudflare は Tunnel での edge 公開のみで DNS 権威ではない）
- **LB IP range**: `192.168.0.240-249`
- **GitHub org**: `yu-min3`
- フォーク時は [`docs/getting-started/configuration.md`](./docs/getting-started/configuration.md) でドメイン等を置換

## User Preferences

- **コマンド出力**: `temp/` に `.sh` + 実行権限で書き出す（詳細: `.claude/rules/collaboration.md` の Script Output Rule が SoT）
- **言語**: 日本語での対話を優先
- **コミット**: 簡潔に 1 文。1 行目 50 文字以内、本文不要（diff を見ればわかる）
