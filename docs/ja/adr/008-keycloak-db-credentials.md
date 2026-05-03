# ADR-008: Keycloak の DB 認証情報を Vault に寄せない

## ステータス

採用済み (Accepted)

## 日付

2026-05-03

## コンテキスト

Phase 2 以降で Vault Database engine による動的 PG credentials が選択肢に上がるが、Keycloak の DB 認証情報をその対象に含めるかは別判断が必要になる。Keycloak は本プラットフォームで「唯一の人間 IdP」であり、Vault / ArgoCD / Grafana / Backstage / 一般アプリの 5 系統が Keycloak の OIDC を見る構造になっている（ADR-005）。

### 依存連鎖

```
[人間] ── OIDC ──→ [Vault]
                      │ (uses for human auth)
                      ↓
                  [Keycloak]
                      │ (needs DB)
                      ↓
                  [Postgres]
```

この構造下で Keycloak の DB 認証情報を Vault dynamic credentials で発行する設計を採用すると、Vault 障害が Keycloak 障害に直結し、結果として 5 系統全てのログインが落ちる。

### 既存実装

`infrastructure/security/keycloak/` 確認結果:

- 専用 Postgres StatefulSet（Keycloak 専用、共用しない）
- Keycloak / Postgres 両方の認証情報が Sealed Secrets で管理
- prod / dev で overlay 分離

→ 既存実装は本 ADR の結論と既に一致している。**変更点はなく、明文化のみ**が本 ADR の目的。

### 要件

1. **Keycloak は infrastructure tier として最も "boring" に保つ**: 余計な依存を持ち込まない
2. **cascade failure を避ける**: Vault 障害が Keycloak 障害に波及しない
3. **DR 時の復旧経路をシンプルに**: Postgres スナップショット + static pw で復元できる

### 検討したパターン

#### パターン A: Sealed Secrets で静的管理（採用、既存実装通り）

専用 Postgres + Sealed Secrets で Keycloak DB 認証情報を管理する。

**メリット:**
- Vault 不在でも Keycloak が動く（独立した failure domain）
- 全停止からの復旧時、Keycloak → Vault の順で起動できる（依存方向が一方向）
- 既存実装そのまま、追加運用ゼロ

**デメリット:**
- DB pw rotation が手動（kubeseal 再実行 + Pod restart）
- audit log が Sealed Secrets controller の K8s event 程度に留まる

#### パターン B: Vault Database engine で動的 credentials

Vault が PG user を 1h TTL 等で動的発行し、ESO 経由で Keycloak Pod に注入する。

**メリット:**
- pw rotation が完全自動
- 各 Keycloak Pod が独立した user で接続でき、audit が user 単位で取れる

**デメリット:**
- **Vault 全停止時に Keycloak が cred refresh 不可で停止** → SSO 全滅
- 全停止からの起動順序が循環（Keycloak は Vault が要る、Vault の OIDC backend は Keycloak が要る）
- DR 時に「Vault が無いと Keycloak が動かない」状態になる

#### パターン C: 段階移行（Phase 1 静的、Phase 3 で dynamic に移行）

Phase 1 は Sealed Secrets、Phase 3 で Vault dynamic に切り替える。

**メリット:**
- 学習段階を経て移行できる

**デメリット:**
- 移行先（パターン B）に cascade failure 問題が残る
- Phase 3 で結局戻すことになる可能性が高い

#### パターン D: Vault KV で静的管理

Sealed Secrets 相当の静的管理を Vault KV に寄せる。

**メリット:**
- audit log が Vault に集約される
- 「全部 Vault」キャッチに整合

**デメリット:**
- Vault 全停止時に Keycloak Pod 再起動で cred 取得できない問題が残る
- Vault dependency を増やしてもメリットは audit log のみ

## 決定

**パターン A（Sealed Secrets で静的管理 + 専用 Postgres）を採用する。永続例外として明文化する。**

| 項目 | 決定 |
|---|---|
| Keycloak の DB 認証情報 | Sealed Secrets で静的管理（永続） |
| Postgres 配置 | Keycloak 専用 Postgres（共用しない） |
| 将来 Vault 移行 | しない（Keycloak は infrastructure tier として boring に保つ） |
| Vault Database engine demo | kensan アプリの Postgres で見せる（Phase 3 / Stage 5） |

### 1. 「全部 Vault に寄せない」原則の Keycloak 版

ADR-007（Vault PKI 不採用）と対をなす方針として、Vault に寄せること自体は目的にしない。Vault dependency を増やす際は **cascade failure の影響範囲が限定されるか** を判断基準にする。

### 2. Secret 在処レジスタの「永続例外」として明示

`secrets-phase1-design.md` の secret 在処レジスタにおいて、Keycloak DB 認証情報は「Sealed Secrets で静的管理（永続）」と記載する。Phase 3 の "Sealed Secret YAML を repo から減らす" 移行対象から **明示的に除外** する。

### 3. Vault Database engine の demo 価値は別アプリで担保

「Vault dynamic credentials を見せる」needs 自体は、kensan アプリ（Streamlit / Jupyter 等）の Postgres を Vault Database engine 配下に置くことで満たす。Keycloak を犠牲にする必要はない。

### 4. DR シナリオ

| 障害 | 対応 |
|---|---|
| Vault 全停止 | Keycloak 影響なし（DB cred 静的）。人間は break-glass userpass で Vault 復旧 |
| Keycloak 全停止 | 人間は Vault userpass / ArgoCD 内蔵 admin / Grafana local admin で凌ぐ |
| Keycloak Postgres 全停止 | snapshot から復元、static pw で再接続 |
| 全停止 | 順次起動: K8s → Sealed Secrets → cert-manager → Istio → **Keycloak Postgres → Keycloak → Vault** |

## 結果

### 良い結果

- Vault 障害が Keycloak 障害に波及しない（独立 failure domain）
- DR 時の起動順序が循環せず、Keycloak → Vault の順で復旧できる
- 既存実装に変更不要（明文化のみで完結）
- Postgres を Keycloak 専用にすることで blast radius が Keycloak に閉じる

### トレードオフ

- DB pw rotation が手動運用（年 1 回など定期実施）
- audit log が Vault に集約されない（Sealed Secrets controller の K8s event レベル）
- 「全部 Vault」キャッチが取れない。意図的な例外として説明する記述コストが発生する

## 関連

- ADR-005: Istio native oauth2 + Keycloak による Phase 1 認証実装（Keycloak が唯一の IdP である前提）
- ADR-007: Vault PKI を採用しない（同じ「全部 Vault に寄せない」原則）
- 設計ソース: `kensan-workspace/projects/kensan-lab/secrets-phase1-design.md` § #4 Keycloak DB の鶏卵問題と決定
- [Keycloak: Database Configuration](https://www.keycloak.org/server/db)
- [HashiCorp Vault Database Secrets Engine](https://developer.hashicorp.com/vault/docs/secrets/databases)
