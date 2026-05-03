# ADR-007: Vault PKI を採用しない（Phase 1〜3）

## ステータス

採用済み (Accepted)

## 日付

2026-05-03

## コンテキスト

`secrets-plan.md` の Phase 2 で Vault を導入する際、Vault PKI engine を有効化して「内部 ingress 用 cert」を発行する案を当初検討していた。しかし kensan-lab の TLS 体制を整理した結果、Vault PKI の operational 余地が現状ほぼ無いことが分かった。

### TLS 担当の現状

| 通信 | 現在の TLS 担当 | Vault PKI 余地 |
|---|---|---|
| 外部ユーザー → Istio Gateway | **Let's Encrypt**（`wildcard-platform-tls` / `wildcard-apps-tls`）| ✕ ブラウザ非信頼 |
| Gateway → Pod (mesh 内) | **Istio 自動 mTLS**（istiod が CA、24h auto-rotate）| ✕ duplicative |
| Pod ↔ Pod (両方 mesh 内) | **Istio 自動 mTLS** | ✕ duplicative |
| Pod → Postgres (両方 mesh 内) | **Istio 自動 mTLS** | ✕ duplicative |
| Pod → mesh 外サービス | アプリ実装次第 | △ 必要時のみ |
| kube-apiserver / etcd | kubeadm 自前管理 | ✕ |

→ Istio が PeerAuthentication: STRICT で intra-cluster mTLS を完全自動化しており、Vault PKI が代替する隙間が無い。

### 要件

1. **既存 TLS 体制を壊さない**: Istio 自前 CA + Let's Encrypt は安定運用中
2. **Vault 導入の operational コストを最小化**: PKI engine を有効化すると root CA / intermediate CA / ロール定義 / cert-manager Vault Issuer 等の運用が増える
3. **将来の選択肢を残す**: audit 集約 / cert revoke 中央集権化等の要件が出たときに導入経路を確保

### 検討したパターン

#### パターン A: Vault PKI を Phase 2 で導入

cert-manager の Vault Issuer 経由で内部 ingress 用 cert を Vault PKI から発行する。

**メリット:**
- 全 cert の発行元が Vault に集約される（audit log も集約）
- cert revoke を Vault で一元管理できる

**デメリット:**
- 内部 ingress 用 cert 自体に需要がない（Istio mTLS で完結している）
- Vault PKI の root CA / intermediate CA / ロール / TTL 設計が新たに必要
- Vault PKI 障害時に cert renewal が止まる

#### パターン B: istio-csr で Vault を Istio root CA にする（将来オプション）

Istio の workload cert（24h SPIFFE cert）を istiod 内蔵 CA ではなく Vault PKI で署名させる。

**メリット:**
- workload cert を含む全 cert が Vault 経由になる
- Single CA reference architecture として完成度が高い

**デメリット:**
- Istio 起動が cert-manager + Vault に依存 → mesh 全体が Vault 健全性に縛られる
- 標準の istiod 内蔵 CA は完全自動・運用ゼロ、それを捨てる選択
- istio-csr は実装例が少なく、homelab 1 人運用での運用知見が薄い

#### パターン C: Vault PKI を入れない（採用）

Istio 自動 mTLS + Let's Encrypt の現体制を維持する。Vault は KV / Database engine / Transit のみで運用する。

**メリット:**
- TLS 体制が既に完結しており、Vault に追加で背負わせるものが無い
- Vault 障害が cert renewal を止めない（cert は cert-manager + Let's Encrypt が独立に管理）
- 運用対象を増やさない（homelab 1 人運用にフィット）

**デメリット:**
- 「全 cert を一元管理する」reference architecture story は描けない
- 将来 audit/compliance 要件が出たときに導入工数が発生する

## 決定

**パターン C を採用する。Phase 1〜3 では Vault PKI engine を有効化しない。**

### 1. 「全部 Vault に寄せない」原則の cert 版として位置付け

ADR-008（Keycloak DB を Vault に寄せない）と対をなす方針として、kensan-lab では **Vault に集約することそれ自体を目的にしない**。Vault が独自価値を出す機能（dynamic credentials / Transit）には寄せるが、既存ツールで完結している領域（cert / Keycloak DB credentials）には寄せない。

### 2. 「内部 ingress 用 cert」は需要が出てから別途検討

`secrets-plan.md` の Phase 2 で「Vault PKI で内部 ingress 用 cert」と記載していた部分は本 ADR で取り下げる。具体的な内部 ingress 需要が出たときに、Istio mTLS で済むか・別途証明書が要るかを再評価する。

### 3. 将来 Vault PKI を入れるトリガー（明文化）

以下のいずれかが発生したら、本 ADR を更新して Vault PKI 導入を再検討する:

- audit / compliance 要件で「全 cert の発行元を一元化」する必要が出た
- cert revoke を中央集権化する必要が出た（漏洩時の即時無効化要件など）
- Vault PKI を活かした具体ユースケース（例: 短命 client cert を mesh 外サービスに提示する）が登場した

導入時のアーキテクチャは **istio-csr 経由で Vault を Istio root CA にする** path を第一候補とする（workload cert を含む全 cert を Vault に集約できるため）。デメリットは「Istio 起動が Vault に依存する」点で、その時点での Vault HA 成熟度を見て判断する。

## 結果

### 良い結果

- TLS 関連の運用対象が増えない（Istio 自動 mTLS + cert-manager + Let's Encrypt の現体制を維持）
- Vault 障害が cert renewal に影響しない（独立した failure domain）
- Vault 導入時の Phase 2 スコープが軽くなる（KV mount + ESO + DB engine だけに集中できる）

### トレードオフ

- 「全 cert を一元管理」のキャッチは描けない。CFP / dev.to ネタとしては「意図的に入れない設計判断」を語る形になる
- 将来 audit/compliance 要件が後付けで来た場合、istio-csr 導入の工数が発生する
- Vault PKI を試したい学習動機があった場合、kensan-lab 内では試験できず別途試験 cluster が必要

## 関連

- ADR-001: TLS Termination Pattern（外部 → Gateway の TLS 終端）
- ADR-008: Keycloak の DB 認証情報を Vault に寄せない（同じ「全部 Vault に寄せない」原則）
- 設計ソース: `kensan-workspace/projects/kensan-lab/secrets-phase1-design.md` § TLS 担当の整理（Vault PKI 不採用の根拠）/ § 将来 Vault PKI を入れるシナリオ: istio-csr 経由
- [istio-csr](https://cert-manager.io/docs/usage/istio-csr/)
- [HashiCorp Vault PKI Secrets Engine](https://developer.hashicorp.com/vault/docs/secrets/pki)
