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
| `/helm-outdated` | 全 chart の鮮度一括チェック（/helm-upgrade の前段） |
| `/new-component <category> <name>` | Scaffold new infra component |
| `/longhorn-health` | Longhorn volume / R2 バックアップ鮮度の一括診断 |
| `/cluster-status` | Quick cluster health check |
| `/troubleshoot <component>` | Diagnose component issues |
| `/cert-check` | Certificate health & expiry check |
| `/secret-health` | Secret 4 方式（Vault/ESO/SealedSecret）の一括健全性チェック |
| `/argocd-sync [app]` | Check sync status & drift |
| `/codex <依頼>` | OpenAI Codex へ委譲（レビュー・セカンドオピニオン・別解生成） |

### SDD (Spec-Driven Development) — infra フロー

仕様を書き上げたら AI が worktree で自律実装し、必須の検証フェーズを通してから draft PR を作る。詳細: [`specs/README.md`](./specs/README.md)。

| Command | Phase | Purpose |
|---------|-------|---------|
| `/sdd-spec <name> [概要]` | 1 | 要件 (what/why) を対話的に作成 → `specs/NNN-<slug>/spec.md` |
| `/sdd-plan <name>` | 2 | 技術設計 (how) → `plan.md`（Helm パターン・chart・namespace 等） |
| `/sdd-tasks <name>` | 3 | 依存順タスク分解 → `tasks.md` |
| `/sdd-impl [name]` | 4 | worktree で自律実装 + 必須検証（静的ゲート + cluster 到達時ライブ検証）→ draft PR を作成 |

> `sdd-` 接頭辞は組み込み `/goal`（"set a goal Claude checks before stopping"）等との衝突回避。`/goal` は「停止前にゴールを確認する番人」で `/sdd-impl` と併用可。
> app 向け SDD は `apps/kensan/` 配下に同じ骨格で別途用意する方針（cwd スコープで同名コマンドが衝突しない）。

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
- **P1 (warn)**: single-arch image 指定（multi-arch manifest list 必須）/ chart version を Application CR の `targetRevision` 以外で管理 / 新規 PVC で `longhorn` 以外の storageClass 指定（local-path は全廃済み）/ stateful データを持つリソース（PVC・StorageClass・RecurringJob 等、prune でデータ消失・再作成不能になるもの）への `Prune=false` annotation 漏れ
- **P2 (info)**: doc-layout 規約違反 / namespace 命名（`app-{name}`）違反 / HTTPRoute の `parentRefs` と Gateway の不整合

## Domain & Network

- **ドメイン**: `yu-min3.com`（DNS 権威は AWS Route53。Cloudflare は Tunnel での edge 公開のみで DNS 権威ではない）
- **第 2 ドメイン**: `yu-mins.com`（Cloudflare Tunnel での外部公開用。`*.yu-mins.com` で argocd/grafana/prometheus/longhorn/backstage を edge 終端 TLS で公開）
- **LB IP range**: `192.168.0.240-249`
- **GitHub org**: `yu-min3`
- フォーク時は [`docs/getting-started/configuration.md`](./docs/getting-started/configuration.md) でドメイン等を置換

## User Preferences

- **コマンド出力**: `temp/` に `.sh` + 実行権限で書き出す（詳細: `.claude/rules/collaboration.md` の Script Output Rule が SoT）
- **言語**: 日本語での対話を優先
- **コミット**: Conventional Commits 形式で簡潔に 1 文。1 行目 50 文字以内、本文・trailer 不要（diff を見ればわかる）
  - 例: `feat(policy): Kyverno 導入`
  - 例: `fix(argocd): overlay path bug 修正`
  - 例: `docs(adr): ADR-012 追加`
  - 例: `chore(helm): cilium 1.18.4 へ更新`
  - 例: `refactor(vault): values 整理`
