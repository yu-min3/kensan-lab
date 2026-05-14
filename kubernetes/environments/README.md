# Environments

Application 用の共有 namespace (`app-prod`) の bootstrap を置く。
**per-app ns 化が完了したら廃止予定**（ADR-006）。

## Application Environments

- `app-prod` — 既存 Backstage scaffolded app の共有 landing zone。新規 app は ADR-006 に従い `app-{name}` flat namespace で作成する

新規 app の namespace 命名は **`app-{name}` flat + 3-axis labels** を採用（[ADR-006](../../docs/adr/006-namespace-naming.md)）。`app-{name}` ns の bootstrap は本ディレクトリではなく、Backstage Software Template が生成する per-app manifest に含める方針。

## 過去の構成 (移行済み)

| 旧 dir | 移行先 | 後継 app |
|---|---|---|
| `kensan-{data,dev,prod}/` | `kubernetes/kensan/{data,dev,prod}/` | ApplicationSet `environments` (path のみ変更、app 名維持) |
| `system-infra/` | `kubernetes/kube-system/` | 単発 Application `kube-system` (旧 `system-infra` の後継) |
| `observability/` | `kubernetes/observability/namespace.yaml` (既存 dead code を生かす) | 単発 Application `monitoring` (旧 `observability` の後継) |
