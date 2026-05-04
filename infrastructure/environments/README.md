# Environments

Application 用の共有 namespace (`app-dev`, `app-prod`) の bootstrap を置く。
**per-app ns 化が完了したら廃止予定**。

## Application Environments

- `app-dev` — 開発環境共有 ns
- `app-prod` — 本番環境共有 ns

## 過去の構成 (移行済み)

| 旧 dir | 移行先 | 後継 app |
|---|---|---|
| `kensan-{data,dev,prod}/` | `infrastructure/kensan/{data,dev,prod}/` | ApplicationSet `environments` (path のみ変更、app 名維持) |
| `system-infra/` | `infrastructure/kube-system/` | 単発 Application `kube-system` (旧 `system-infra` の後継) |
| `observability/` | `infrastructure/observability/namespace.yaml` (既存 dead code を生かす) | 単発 Application `monitoring` (旧 `observability` の後継) |
