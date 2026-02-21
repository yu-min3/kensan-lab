# 環境固有の設定変更ガイド

このリポジトリを別環境で使用する際に変更が必要な設定項目をまとめています。

> **重要**: このリポジトリには特定の環境に依存する設定が含まれています。
> 以下の項目をすべて確認し、自分の環境に合わせて変更してください。

## クイック設定チェックリスト

- [ ] **ドメイン名**の変更（TLS証明書、HTTPRoute、Keycloak等）
- [ ] **GitHub組織名・ユーザー名**の変更（Argo CD Application CR、GHCR認証情報）
- [ ] **LoadBalancer IP範囲**の変更（Cilium LoadBalancer IP Pool）
- [ ] **ネットワークインターフェース**の変更（Cilium L2 Announcement）
- [ ] **TLS証明書用の認証情報**の設定（Cert-Manager + DNS Provider）
- [ ] **Sealed Secrets**の再生成（新しいクラスター用）
- [ ] **Alertmanager Slack Webhook**の設定（通知を有効化する場合）

## 必須変更項目

### 1. ドメイン名

**対象コンポーネント:** Keycloak, Backstage, Prometheus, Grafana, Argo CD

このリポジトリでは例として `example.com` ドメインを使用していますが、**実際のドメイン名に変更する必要があります**。

#### 変更が必要なファイル

| コンポーネント | ファイルパス | 設定項目 |
|--------------|------------|---------|
| **Keycloak (Prod)** | `base-infra/keycloak/overlays/prod/httproute.yaml` | `hostnames: ["auth.example.com"]` |
| **Keycloak (Prod)** | `base-infra/keycloak/overlays/prod/keycloak-env-patch.yaml` | `KC_HOSTNAME: auth.example.com` |
| **Keycloak (Dev)** | `base-infra/keycloak/overlays/dev/httproute.yaml` | `hostnames: ["auth-dev.example.com"]` |
| **Backstage** | `base-infra/backstage/base/httproute.yaml` | `hostnames: ["backstage.example.com"]` |
| **Prometheus** | `base-infra/prometheus/httproute-prometheus.yaml` | `hostnames: ["prometheus.example.com"]` |
| **Grafana** | `base-infra/prometheus/httproute-grafana.yaml` | `hostnames: ["grafana.example.com"]` |
| **Argo CD** | `base-infra/argocd/httproute.yaml` | `hostnames: ["argocd.example.com"]` |
| **Cert-Manager Certificates** | `base-infra/cert-manager/wildcard-certificate-*.yaml` | `dnsNames: ["*.example.com"]` |

#### 変更例

```yaml
# Keycloak Production HTTPRoute
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: keycloak
  namespace: platform-auth-prod
spec:
  hostnames:
  - "auth.your-domain.com"  # ← ここを変更

---
# Keycloak ConfigMap Patch
data:
  KC_HOSTNAME: auth.your-domain.com  # ← ここを変更
```

#### 一括検索・置換の方法

> **重要**: ドメイン名は多数のYAMLファイルにハードコーディングされています。
> 手動で1つずつ変更するのは非効率的なので、エディタの一括置換機能を使用することを強く推奨します。

**方法1: VSCode（Visual Studio Code）を使用**

1. VSCodeでこのリポジトリを開く
2. `Ctrl+Shift+H`（Mac: `Cmd+Shift+H`）で検索・置換パネルを開く
3. 検索ボックスに `example\.com` を入力（正規表現を有効化）
4. 置換ボックスに `your-domain.com` を入力
5. "ファイル内を検索" で `base-infra/` を指定
6. "すべて置換" をクリック前にプレビュー確認

**方法2: sedコマンドを使用（Linux/Mac）**

```bash
# 1. 現在のドメイン名を確認
grep -r "example\.com" base-infra/ --include="*.yaml"

# 2. バックアップを作成（推奨）
cp -r base-infra base-infra.backup

# 3. 一括置換を実行
find base-infra/ -type f -name "*.yaml" -exec sed -i 's/example\.com/your-domain.com/g' {} +

# 4. 変更を確認
git diff base-infra/
```

**方法3: PowerShell（Windows）**

```powershell
# 現在のドメイン名を確認
Get-ChildItem -Path base-infra -Recurse -Filter *.yaml | Select-String "example\.com"

# 一括置換
Get-ChildItem -Path base-infra -Recurse -Filter *.yaml | ForEach-Object {
    (Get-Content $_.FullName) -replace 'example\.com', 'your-domain.com' | Set-Content $_.FullName
}
```

**検索すべきパターン:**

ドメイン名が使用されている可能性がある箇所を検索する際は、以下のパターンを使用してください：

- `example.com` - 基本的なドメイン名
- `*.example.com` - ワイルドカードドメイン
- `auth.example.com`, `backstage.example.com` 等 - サブドメイン
- `https://example.com` - URL形式

**注意事項:**

- `github.com` は置換対象外（GitHubのURLは変更不要）
- コメント内のドメイン名も置換されるので、必要に応じて調整

---

### 2. GitHub組織名・ユーザー名

**対象:** Argo CD Application CRs, GHCR認証情報, Backstageイメージ

#### Argo CD Application CRs

すべてのArgo CD Application CRで、GitリポジトリのURLを変更します。

**変更が必要なファイル:**
- `base-infra/argocd/applications/*.yaml`（全Applicationファイル）
- `base-infra/argocd/root-apps/platform-root-app.yaml`

```yaml
# 変更前
spec:
  source:
    repoURL: 'https://github.com/your-org/goldship'  # ← ここを変更
    targetRevision: HEAD
    path: base-infra/prometheus
```

**一括置換の方法:**

> **重要**: GitHub組織名/ユーザー名もYAMLファイルにハードコーディングされています。
> エディタの一括置換機能を使用することを推奨します。

**方法1: VSCodeを使用**

1. `Ctrl+Shift+H`（Mac: `Cmd+Shift+H`）で検索・置換パネルを開く
2. 検索: `github\.com/your-org`（正規表現を有効化）
3. 置換: `github.com/YOUR_GITHUB_ORG`
4. ファイル: `base-infra/argocd/` を指定
5. "すべて置換" を実行

**方法2: sedコマンド（Linux/Mac）**

```bash
# 1. 現在のGitHub組織名を確認
grep -r "github.com/your-org" base-infra/argocd/ --include="*.yaml"

# 2. 一括置換
find base-infra/argocd/ -type f -name "*.yaml" -exec sed -i 's|github.com/your-org|github.com/YOUR_ORG|g' {} +

# 3. 変更を確認
git diff base-infra/argocd/
```

**方法3: PowerShell（Windows）**

```powershell
# 一括置換
Get-ChildItem -Path base-infra\argocd -Recurse -Filter *.yaml | ForEach-Object {
    (Get-Content $_.FullName) -replace 'github\.com/your-org', 'github.com/YOUR_ORG' | Set-Content $_.FullName
}
```

#### GHCRイメージ名（Backstage）

Backstageのコンテナイメージ名を変更します。

**ファイル:** `base-infra/backstage/overlays/prod/kustomization.yaml`

```yaml
images:
- name: ghcr.io/your-org/backstage  # ← ここを変更
  newTag: v0.0.5
```

#### GHCR Pull Secrets

**ファイル:**
- `base-infra/app-dev/ghcr-pull-secret.yaml`（Sealed Secret）
- `base-infra/app-prod/ghcr-pull-secret.yaml`（Sealed Secret）

これらは暗号化されているため、**新しいクラスターでは再生成が必要です**（後述の「Sealed Secrets」セクション参照）。

---

### 3. LoadBalancer IP範囲

**対象:** Cilium LoadBalancer IP Pool

クラスターが動作するネットワーク環境に応じて、LoadBalancer用のIP範囲を設定します。

**ファイル:** `base-infra/cilium/lb-ippool.yaml`

```yaml
apiVersion: cilium.io/v2alpha1
kind: CiliumLoadBalancerIPPool
metadata:
  name: main-pool
spec:
  blocks:
  - cidr: 192.168.1.240/28  # ← 自分のネットワークに合わせて変更（例: 192.168.1.240-255）
```

**注意事項:**
- DHCPサーバーの割り当て範囲と**重複しないように設定**してください
- ルーターやファイアウォールの設定も確認してください
- 最低でも10個以上のIPアドレスを確保することを推奨

**IP範囲の確認:**

```bash
# 現在のLoadBalancer IP範囲を確認
kubectl get ciliumloadbalancerippools -o yaml

# LoadBalancer Serviceに割り当てられたIPを確認
kubectl get svc -A -o wide | grep LoadBalancer
```

---

### 4. ネットワークインターフェース

**対象:** Cilium L2 Announcement Policy

LoadBalancer IPを外部に通知するネットワークインターフェースを指定します。

**ファイル:** `base-infra/cilium/cilium.yaml`（Helm values内）

```yaml
# Cilium Helm values
l2announcements:
  enabled: true
devices: wlan0  # ← 自分の環境のネットワークインターフェースに変更（例: eth0, enp0s3等）
```

**インターフェース名の確認方法:**

```bash
# ノードにSSHしてインターフェース名を確認
ip addr show

# 例:
# 1: lo: <LOOPBACK,UP,LOWER_UP> ...
# 2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> ...  ← これを使用
# 3: wlan0: <BROADCAST,MULTICAST,UP,LOWER_UP> ...
```

---

### 5. TLS証明書とDNSプロバイダー認証情報

**対象:** Cert-Manager + Let's Encrypt

#### DNS-01チャレンジ用認証情報（AWS Route53の例）

**ファイル:** `base-infra/cert-manager/route53-credentials-sealed.yaml`（Sealed Secret）

これは暗号化されているため、**新しいクラスターでは再生成が必要です**。

**再生成手順:**

```bash
# 1. Raw secretを作成
cat > temp/route53-credentials-raw.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: route53-credentials
  namespace: cert-manager
type: Opaque
stringData:
  secret-access-key: YOUR_AWS_SECRET_ACCESS_KEY
EOF

# 2. Sealed Secretに変換
kubeseal --format=yaml < temp/route53-credentials-raw.yaml \
  > base-infra/cert-manager/route53-credentials-sealed.yaml

# 3. 適用
kubectl apply -f base-infra/cert-manager/route53-credentials-sealed.yaml
```

#### ClusterIssuer設定

**ファイル:** `base-infra/cert-manager/clusterissuer.yaml`

```yaml
spec:
  acme:
    email: admin@your-domain.com  # ← Let's Encrypt通知用メールアドレスを変更
    server: https://acme-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - dns01:
        route53:
          region: ap-northeast-1  # ← AWSリージョンを変更（必要に応じて）
          accessKeyID: YOUR_ACCESS_KEY_ID  # ← AWS Access Key IDを変更
```

#### Wildcard証明書

**ファイル:**
- `base-infra/cert-manager/wildcard-certificate-platform.yaml`
- `base-infra/cert-manager/wildcard-certificate-apps.yaml`

```yaml
spec:
  dnsNames:
  - "*.your-domain.com"  # ← ワイルドカードドメインを変更
  - "your-domain.com"
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
```

---

### 6. Sealed Secrets

新しいクラスターでは、Sealed Secrets Controllerが異なる暗号化キーを使用するため、**既存のSealed Secretsは復号化できません**。

#### 再生成が必要なSealed Secrets

| ファイルパス | 説明 |
|------------|------|
| `base-infra/app-dev/ghcr-pull-secret.yaml` | GHCR認証情報（開発環境） |
| `base-infra/app-prod/ghcr-pull-secret.yaml` | GHCR認証情報（本番環境） |
| `base-infra/backstage/base/ghcr-pull-secret.yaml` | Backstage用GHCR認証情報 |
| `base-infra/backstage/base/postgresql-secret.yaml` | PostgreSQL認証情報 |
| `base-infra/backstage/base/backstage-secret.yaml` | Backstage環境変数 |
| `base-infra/keycloak/overlays/prod/postgresql-sealed-secret.yaml` | Keycloak PostgreSQL（本番） |
| `base-infra/keycloak/overlays/prod/keycloak-sealed-secret.yaml` | Keycloak Admin認証情報（本番） |
| `base-infra/keycloak/overlays/dev/postgresql-sealed-secret.yaml` | Keycloak PostgreSQL（開発） |
| `base-infra/keycloak/overlays/dev/keycloak-sealed-secret.yaml` | Keycloak Admin認証情報（開発） |
| `base-infra/prometheus/grafana-sealed-secret.yaml` | Grafana Admin認証情報 |
| `infrastructure/observability/prometheus/resources/alertmanager-slack-sealed-secret.yaml` | Alertmanager Slack Webhook URL |
| `base-infra/cert-manager/route53-credentials-sealed.yaml` | AWS Route53認証情報 |

#### 一般的な再生成手順

```bash
# 1. Sealed Secrets Controllerをデプロイ
kubectl apply -f base-infra/sealed-secret/controller.yaml

# 2. Raw secretを作成（temp/ディレクトリ内）
kubectl create secret docker-registry ghcr-pull-secret \
  --docker-server=ghcr.io \
  --docker-username=YOUR_GITHUB_USERNAME \
  --docker-password=YOUR_GITHUB_PAT \
  --docker-email=YOUR_EMAIL \
  --namespace=app-prod \
  --dry-run=client -o yaml > temp/ghcr-secret-raw-prod.yaml

# 3. Sealed Secretに変換
kubeseal --format=yaml < temp/ghcr-secret-raw-prod.yaml \
  > base-infra/app-prod/ghcr-pull-secret.yaml

# 4. 適用
kubectl apply -f base-infra/app-prod/ghcr-pull-secret.yaml

# 5. temp/ディレクトリ内のraw secretは削除（git-ignored）
rm temp/ghcr-secret-raw-prod.yaml
```

---

## オプション変更項目

### StorageClass（Local Path Provisioner）

**ファイル:** `base-infra/local-path-provisioner/local-path-provisioner.yaml`

デフォルトのストレージパスを変更する場合:

```yaml
data:
  config.json: |-
    {
      "nodePathMap":[
        {
          "node":"DEFAULT_PATH_FOR_NON_LISTED_NODES",
          "paths":["/opt/local-path-provisioner"]  # ← ストレージパスを変更
        }
      ]
    }
```

### Keycloak Admin認証情報

開発・本番環境のKeycloak管理者パスワードを変更する場合は、Sealed Secretを再生成してください。

```bash
# Keycloak Admin Secret例
kubectl create secret generic keycloak-secret \
  --namespace=platform-auth-prod \
  --from-literal=KEYCLOAK_ADMIN=admin \
  --from-literal=KEYCLOAK_ADMIN_PASSWORD=STRONG_PASSWORD_HERE \
  --dry-run=client -o yaml > temp/keycloak-secret-raw-prod.yaml

kubeseal --format=yaml < temp/keycloak-secret-raw-prod.yaml \
  > base-infra/keycloak/overlays/prod/keycloak-sealed-secret.yaml
```

---

## 変更後の確認

すべての変更を適用した後、以下を確認してください:

### 1. Kustomizeビルドの検証

```bash
# Kustomizeベースのコンポーネントをビルドして構文エラーを確認
kubectl kustomize base-infra/keycloak/overlays/prod/ > /dev/null
kubectl kustomize base-infra/keycloak/overlays/dev/ > /dev/null
kubectl kustomize base-infra/backstage/overlays/prod/ > /dev/null

echo "All Kustomize builds passed!"
```

### 2. 個人情報の残留確認

```bash
# 例として使用されていたドメイン名が残っていないか確認
grep -r "example\.com" base-infra/ || echo "No example.com found (OK)"

# 例として使用されていたGitHub組織名が残っていないか確認
grep -r "github.com/your-org" base-infra/ || echo "No placeholder org found (OK)"
```

### 3. Argo CD Application CRの確認

```bash
# すべてのApplicationのrepoURLが正しいか確認
grep -h "repoURL:" base-infra/argocd/applications/*.yaml | sort -u
```

---

## トラブルシューティング

### ドメイン名の変更を忘れた場合

**症状:** Keycloakが起動しても、設定されたホスト名と実際のアクセスURLが一致せず、アクセスできない。

**解決策:**
1. 上記「1. ドメイン名」セクションを参照して修正
2. Keycloak Podを再起動: `kubectl rollout restart deployment keycloak -n platform-auth-prod`

### LoadBalancer IPが割り当てられない

**症状:** LoadBalancer ServiceのEXTERNAL-IPが `<pending>` のまま。

**原因:**
- Cilium LoadBalancer IP Poolの範囲が不正
- ネットワークインターフェース名が間違っている
- DHCPとIP範囲が重複している

**解決策:**

```bash
# Cilium LoadBalancer IP Poolを確認
kubectl get ciliumloadbalancerippools -o yaml

# Cilium L2 Announcement Policyを確認
kubectl get ciliuml2announcementpolicies -o yaml

# Ciliumログを確認
kubectl logs -n kube-system -l k8s-app=cilium | grep -i "loadbalancer\|l2"
```

### Sealed Secretsが復号化されない

**症状:** PodがImagePullBackOffやSecretNotFoundでエラーになる。

**原因:** 別クラスターで生成されたSealed Secretsを使用している。

**解決策:**
1. 「6. Sealed Secrets」セクションを参照して全Sealed Secretsを再生成
2. `temp/`ディレクトリ内のraw secretは絶対にコミットしない

### TLS証明書が発行されない

**症状:** HTTPSアクセスができない、または証明書エラーが表示される。

**原因:**
- DNS-01チャレンジの認証情報が間違っている
- DNSレコードが設定されていない
- Let's Encrypt APIレート制限に達している

**解決策:**

```bash
# Certificate Requestの状態を確認
kubectl get certificaterequest -A

# Certificate発行ログを確認
kubectl logs -n cert-manager -l app=cert-manager

# CertificateのStatusを確認
kubectl describe certificate -n istio-system wildcard-tls
```

---

## まとめ

このガイドで説明した設定項目を変更することで、このリポジトリを自分の環境で使用できるようになります。

**チェックリストの再確認:**
- [ ] すべてのドメイン名を変更した
- [ ] GitHub組織名・ユーザー名を変更した
- [ ] LoadBalancer IP範囲を環境に合わせた
- [ ] ネットワークインターフェース名を確認・変更した
- [ ] TLS証明書用の認証情報を設定した
- [ ] すべてのSealed Secretsを再生成した
- [ ] Alertmanager Slack Webhook を設定した（通知を使用する場合）
- [ ] Kustomizeビルドが成功することを確認した
- [ ] 個人情報が残っていないことを確認した

これで、自分の環境でこのプラットフォームを運用する準備が整いました！
