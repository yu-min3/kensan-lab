# ADR-001: TLS終端パターンの選択

## ステータス

採用済み (Accepted)

## 日付

2025-11-07

## コンテキスト

プラットフォームインフラサービス（Argo CD、Grafana、Prometheus、Keycloak、Hubble UIなど）を外部に公開する際、TLS終端をどこで行うかを決定する必要がある。

### TLS終端の3つのパターン

#### 1. Edge Termination（エッジ終端）

```
[ブラウザ] --HTTPS--> [Gateway] --HTTP--> [Backend]
                      ↑ TLS終端
```

- **特徴**: GatewayでTLS証明書を管理し、バックエンドにはHTTPで転送
- **メリット**:
  - 証明書管理が1箇所（Gateway）で集中管理
  - バックエンドのTLS設定不要
  - 複数サービスで証明書を共有可能
  - パフォーマンスが良い（暗号化/復号化は1回）
- **デメリット**:
  - Gateway→Backend間は平文（クラスター内）

#### 2. Passthrough Termination（パススルー）

```
[ブラウザ] --HTTPS--> [Gateway] --HTTPS--> [Backend]
                      ↑ 暗号化のまま転送  ↑ TLS終端
```

- **特徴**: Gatewayは暗号化トラフィックをそのまま転送、バックエンドで終端
- **メリット**:
  - End-to-endで暗号化
  - バックエンドが証明書を完全制御
- **デメリット**:
  - 各バックエンドに証明書が必要
  - GatewayでL7ルーティング不可（暗号化されているため）
  - HTTPヘッダー操作・認証処理不可

#### 3. Re-encryption（再暗号化）

```
[ブラウザ] --HTTPS--> [Gateway] --HTTPS--> [Backend]
                      ↑ TLS終端        ↑ TLS終端
                      ↓ TLS再暗号化
```

- **特徴**: Gatewayで復号化後、再度暗号化してバックエンドに転送
- **メリット**:
  - End-to-endで暗号化
  - GatewayでL7処理可能（ヘッダー挿入、認証など）
  - Gateway→Backend間も暗号化
- **デメリット**:
  - 最も複雑
  - パフォーマンスオーバーヘッド（2回の暗号化/復号化）
  - Gateway、Backend両方に証明書必要
  - **Istio SidecarがService Meshとして必須**（重い）

## 決定事項

**Edge Termination（エッジ終端）を採用する**

すべてのプラットフォームインフラサービスで、Istio GatewayによるEdge Terminationパターンを使用する。

### 適用対象サービス

- Argo CD
- Grafana
- Prometheus
- Keycloak (Prod/Dev)
- Hubble UI
- 将来追加されるプラットフォームサービス

### 実装方法

1. **Gateway側**:
   - Istio Gatewayで Let's Encrypt証明書を使用（cert-manager自動管理）
   - `*.platform.your-org.com` ワイルドカード証明書
   - TLS終端（`tls.mode: Terminate`）

2. **Backend側**:
   - サービスはHTTPで動作（insecure mode）
   - 例: Argo CD `server.insecure: "true"`
   - ClusterIP Serviceで公開

3. **HTTPRoute**:
   - バックエンドサービスのHTTPポート（通常80番）を指定

## 理由

### Edge Terminationを選択した理由

1. **証明書管理の簡素化**
   - Let's Encryptの証明書を1箇所（cert-manager + Gateway）で管理
   - 各バックエンドサービスに証明書を配布する必要がない
   - 証明書更新が自動化され、影響範囲が限定的

2. **パフォーマンス**
   - 暗号化/復号化が1回のみ（Gatewayのみ）
   - Bare-metal Raspberry Pi環境での省リソース

3. **運用性**
   - バックエンドサービスのTLS設定が不要
   - Helmチャートや既存マニフェストをそのまま利用可能
   - トラブルシューティングが容易

4. **クラスター内通信の信頼性**
   - Kubernetes クラスター内は信頼できるネットワークとして扱う
   - NetworkPolicyで名前空間間の通信を制御可能
   - 物理的に隔離されたベアメタル環境

### Re-encryptionを見送った理由

1. **Istio Sidecar注入が必須**
   - Service Mesh（mTLS）を有効にする必要がある
   - 各PodにEnvoy Sidecarが注入される
   - **リソースオーバーヘッドが大きい**（Raspberry Pi環境では重い）

2. **複雑性の増加**
   - 証明書を2箇所で管理（Gateway + Backend）
   - デバッグが困難
   - パフォーマンスオーバーヘッド（2回の暗号化/復号化）

3. **本環境での必要性が低い**
   - クラスター内は物理的に隔離されたベアメタル環境
   - NetworkPolicyで十分な制御が可能
   - プラットフォームインフラサービスは管理者のみがアクセス

### Passthroughを見送った理由

1. **L7ルーティング不可**
   - HTTPヘッダーベースのルーティングができない
   - 将来的なKeycloak JWT認証統合が困難

2. **証明書管理の複雑化**
   - 各バックエンドに証明書が必要
   - Let's Encryptの証明書を個別に管理する必要

## 結果

### 実装上の注意点

1. **バックエンドサービスの設定**

   各サービスで「TLS終端はプロキシが処理」と明示的に設定する必要がある：

   - **Argo CD**: `server.insecure: "true"` in ConfigMap
   - **Grafana**: デフォルトでHTTP対応（追加設定不要）
   - **Prometheus**: デフォルトでHTTP対応（追加設定不要）
   - **Keycloak**: `KC_PROXY=edge` 環境変数

2. **リダイレクトループの回避**

   バックエンドサービスがHTTPS強制リダイレクトを行う場合、無限ループが発生する。
   必ず insecure/edge mode を有効にすること。

3. **証明書の範囲**

   - `*.platform.your-org.com`: プラットフォームインフラ用
   - `*.app.your-org.com`: アプリケーション用（将来）

   Let's Encryptのワイルドカード証明書は1レベルのサブドメインのみカバーするため、
   階層的なサブドメイン構造に応じて証明書を分ける。

### トレードオフ

**セキュリティ vs パフォーマンス/運用性**

- ✅ 採用: クラスター内は平文、運用が容易、パフォーマンスが良い
- ❌ 不採用: End-to-end暗号化、複雑、重い

本環境では、以下の理由により Edge Termination が適切：
- 物理的に隔離されたベアメタル環境
- Raspberry Pi のリソース制約
- プラットフォームインフラサービスは管理者のみがアクセス
- NetworkPolicyによる名前空間間の通信制御

### 将来的な再検討の条件

以下の場合、Re-encryptionパターンへの移行を検討する：

1. **コンプライアンス要件**
   - 規制によりEnd-to-end暗号化が必須となった場合

2. **マルチテナント環境**
   - 複数組織のアプリケーションを同一クラスターでホストする場合

3. **ゼロトラストネットワーク**
   - クラスター内通信も暗号化が必要なセキュリティポリシーを採用する場合

4. **ハードウェアアップグレード**
   - より高性能なノードに移行し、Istio Sidecarのオーバーヘッドが許容範囲内になった場合

## 参考資料

- [Istio Gateway API - TLS Configuration](https://istio.io/latest/docs/tasks/traffic-management/ingress/gateway-api/#configuring-tls)
- [Argo CD - Running Argo CD behind a proxy](https://argo-cd.readthedocs.io/en/stable/operator-manual/ingress/)
- [Keycloak - Using a reverse proxy](https://www.keycloak.org/server/reverseproxy)
- [Let's Encrypt - Wildcard Certificates](https://letsencrypt.org/docs/faq/#does-let-s-encrypt-issue-wildcard-certificates)
