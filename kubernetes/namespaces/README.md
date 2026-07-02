# Namespaces

namespace ライフサイクル管理の置き場（旧 `environments/`。dev/prod 分離廃止後、実体が「ns の bootstrap と label 契約の維持」に変わったため改名）。

## 構成

ns 管理は 2 種類に分かれる:

| 種類 | 置き場 | 例 |
|---|---|---|
| **shared landing zone** | 本ディレクトリ（`app-prod/` — ns + SA + pull-secret） | `app-prod` |
| **component の ns-lifecycle** | `namespace.yaml` を**コンポーネント本体ディレクトリに同居**させ、`kubernetes/argocd/applications/namespaces/<name>/app.yaml` が `directory.include: 'namespace.yaml'` で抜く | `kubernetes/secrets/reloader/namespace.yaml`、`kubernetes/policy/namespace.yaml` |

統一パターンの意図: ns の定義が component の隣にあり（発見性）、Application は label 契約（[ADR-014](../../docs/adr/014-namespace-naming-label-contract-v2.md)、Kyverno `ns-label-contract` が enforce）込みで ns を SSA adopt する。component 本体の app は `CreateNamespace=false`（ns の所有者は lifecycle app に一本化）。

## 運用ノート

- **Application 名の `-namespace` suffix は不統一のまま**（`reloader-namespace` vs `kube-system` 等）。ArgoCD の Application rename = 旧 prune + 新 create であり、ns 巻き添え削除の事故リスクに見合わないため意図的に据え置き（ownership-transfer の手順を踏めば可能だが優先度低）
- `sealed-secrets-namespace` / `local-path-storage-namespace` は **finalizer なし + Prune=false**（sealing key / legacy PVC の保護。各 app.yaml のヘッダコメント参照）
- 同居ディレクトリに他の manifest がある場合、component 側 app に `directory.exclude: 'namespace.yaml'` が必要（例: `applications/secrets/sealed-secrets/app.yaml`）

## app-prod (shared landing zone)

既存 Backstage scaffolded app の共有 landing zone。新規 app は [ADR-006](../../docs/adr/006-namespace-naming.md) に従い `app-{name}` flat namespace を per-app manifest で持つ（Backstage Software Template が生成）。**空になり次第 `app-prod` は削除候補**。
