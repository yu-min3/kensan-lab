# ADR-006: Application Namespace 命名規約 (`app-{name}` flat + label 3 軸)

## ステータス

採用済み (Accepted)

## 日付

2026-05-03

## コンテキスト

`environment-separation.md` で定義済みの三層モデル（Infrastructure / Environment / Application）のうち、Application Layer の namespace 命名はこれまで `app-prod-<name>` / `app-dev-<name>` の env-first パターンを採っていた。

Phase 1 の認証/認可設計（ADR-005）と並行して、AuthorizationPolicy / NetworkPolicy / ArgoCD project scope を team 軸で切れる構造が必要になった。env-first 命名のままでは:

1. namespace 名そのものに env と app しか含まれず、team 軸を表現できない
2. 機械処理（Gateway API allowedRoutes selector など）に namespace 名 pattern を使うのは、命名変更時の影響範囲が広く脆い

### 既存実装の確認

`infrastructure/network/istio/resources/gateway-platform.yaml` 等の既存 Gateway は **既に label-based selector** で namespace を制御している:

```yaml
allowedRoutes:
  namespaces:
    from: Selector
    selector:
      matchLabels:
        kensan-lab.platform/environment: infrastructure
```

→ 機械処理は label に依存しており、namespace 名 pattern には依存していない。**命名規約を変えても Gateway YAML を触る必要がない**。

### 要件

1. **team 軸を表現できること**: AuthorizationPolicy / NetworkPolicy で team 単位の制御が可能
2. **既存 Gateway リソースを変更しない**: label-based selector のため、namespace 命名と独立に運用できるはず
3. **homelab 1 人運用にフィット**: team 強調が強すぎない命名（PE / AD 分離が将来現実化したときに耐える程度）
4. **Backstage 整合**: 将来 Backstage Software Template の owner 制御を本格化したとき、team label で参照できる

### 検討したパターン

#### パターン A: 現状維持（`app-{env}-{name}`）

例: `app-prod-streamlit`, `app-dev-streamlit`

**メリット:**
- 既存リソースに変更不要
- env が namespace 名から自明

**デメリット:**
- team 軸を表現できない
- env 毎に namespace を分ける必要があり、命名が冗長

#### パターン B: team-first (`app-{team}-{name}`)

例: `app-team-a-streamlit`, `app-team-b-iceberg-ui`

**メリット:**
- team 軸が namespace 名から自明
- AuthorizationPolicy で team 単位の操作がしやすく見える

**デメリット:**
- homelab 1 人運用では team は実体がなく、命名のオーバーヘッドが大きい
- env を namespace 名から外すと、prod/dev の取り違え事故リスクが上がる

#### パターン C: flat (`app-{name}`) + label 3 軸（採用）

例: `app-streamlit`, `app-iceberg-ui`

namespace 名は app 名のみ。env / team は label で表現する。

**メリット:**
- namespace 名がシンプルで人間可読
- team / env / app の 3 軸を独立に label で切れる
- 「team を後から追加」「env をマージ」等の構造変更が rename 不要
- ADR-005 の AuthorizationPolicy が group claim と label の組み合わせで表現できる

**デメリット:**
- env が namespace 名から見えない（label を参照する必要あり）
- 既存の `app-prod-<name>` 命名との混在期間が発生する

## 決定

**パターン C（`app-{name}` flat + label 3 軸）を採用する。**

### 1. 命名規約

```
app-{name}                  ← アプリ用（flat）
  例: app-streamlit
      app-iceberg-ui
      app-jupyterhub

platform-{component}        ← プラットフォーム用（既存維持）
  例: platform-keycloak
      platform-vault
      platform-argocd
```

### 2. Label 3 軸

| Label | 値の例 | 用途 | 既存？ |
|---|---|---|---|
| `kensan-lab.platform/environment` | `production` / `development` / `infrastructure` | Gateway API allowedRoutes selector | **既存** |
| `kensan-lab.platform/team` | `team-a` / `team-b` / `platform` | AuthorizationPolicy / NetworkPolicy / ArgoCD project scope | **新規** |
| `kensan-lab.platform/app` | `streamlit` / `iceberg-ui` / ... | 識別用（任意） | 新規 |

### 3. ArgoCD Project は Phase 1 では既存維持

既存:
- `platform-project`（Infrastructure）
- `app-project-prod` / `app-project-dev`（env 軸）

team 軸 Project（`app-project-team-a` 等）への分割は Phase 2 以降に再検討する。Phase 1 では既存 Project の `destinations` に新命名 namespace（`app-{name}`）を追加するのみで対応する。

### 4. 移行スケジュール

| 対象 | Phase 1 移行 |
|---|---|
| 新規アプリ | 本規約で `app-{name}` で作成 |
| 既存 `app-prod-<name>` / `app-dev-<name>` | **急がない**。次回大改修時に rename。並行運用許容 |
| `app-prod` / `app-dev` namespace（env 共用 ns）| そのまま残す |

`app-prod` / `app-dev` は env 共用 namespace として残し続ける（既存アプリの受け皿）。これらが空になった時点で削除を検討する。

## 結果

### 良い結果

- namespace 名がシンプルで人間可読（`app-streamlit` だけ覚えればよい）
- team 軸が後付けで導入できる（label のみで切れるため、後から `kensan-lab.platform/team` を追加すれば済む）
- 既存 Gateway リソース（label-based selector）を一切触らずに移行可能
- ADR-005 の Gateway-level AuthorizationPolicy が label と group claim の組み合わせで宣言的に書ける

### トレードオフ

- env が namespace 名から見えない。`kubectl get ns -L kensan-lab.platform/environment` のように label 表示前提の運用になる
- 既存 `app-prod-<name>` との並行期間が発生する。命名統一には時間がかかる
- `kensan-lab.platform/team` label を全リソースに付け忘れると AuthorizationPolicy が機能しない。Backstage Software Template / 新規 namespace 作成手順で必須項目化する必要がある

## 関連

- ADR-005: Istio native oauth2 + Keycloak による Phase 1 認証実装（label を AuthorizationPolicy で参照）
- `.claude/rules/environment-separation.md`（三層モデルの定義）
- 設計ソース: `kensan-workspace/projects/kensan-lab/secrets-phase1-design.md` § Namespace 分離戦略
