# 環境固有の設定変更ガイド

このリポジトリを別環境で使用する際に変更が必要な設定項目をまとめています。

## 必須変更項目

### 1. ドメイン名

このリポジトリでは `yu-min3.com` ドメインを使用していますが、環境に応じて変更が必要です。

#### Keycloak

**ファイル:**
- `base-infra/keycloak/overlays/prod/httproute.yaml`
- `base-infra/keycloak/overlays/prod/keycloak-env-patch.yaml`
- `base-infra/keycloak/overlays/dev/httproute.yaml`

**変更箇所:**

```yaml
# Production
hostnames:
- "auth.yu-min3.com"  # ← 自分のドメインに変更

KC_HOSTNAME: auth.yu-min3.com  # ← 自分のドメインに変更

# Development
hostnames:
- "auth-dev.yu-min3.com"  # ← 自分のドメインに変更
```

### 2. IPアドレス範囲

**ファイル:**
- `base-infra/cilium/lb-ippool.yaml`

**変更箇所:**

```yaml
# Cilium LoadBalancer IP範囲
- 192.168.0.240-192.168.0.249  # ← 自分のネットワークに合わせて変更
```

**注意:** DHCPの割り当て範囲と重複しないように設定してください。

### 3. ノードのIPアドレス

**ファイル:**
- CLAUDE.md (ドキュメント用)

**変更箇所:**

```yaml
Master: 192.168.0.107  # ← 自分のMasterノードIPに変更
Pod CIDR: 10.244.0.0/16  # ← 必要に応じて変更
```

### 4. ネットワークインターフェース

**ファイル:**
- `base-infra/cilium/cilium.yaml`

**変更箇所:**

```yaml
devices: wlan0  # ← 自分の環境のネットワークインターフェースに変更
```

インターフェース名は `ip addr` コマンドで確認できます。

### 5. GitHubリポジトリURL

Argo CD ApplicationのソースURLを変更する必要があります。

**ファイル:**
- `base-infra/argocd/applications/*.yaml`

**変更箇所:**

```yaml
spec:
  source:
    repoURL: 'https://github.com/yu-min3/k8s-platform-config'  # ← 自分のリポジトリURLに変更
```

### 6. TLS証明書

Istio Gatewayで使用する証明書を自分のドメイン用に発行・配置してください。

**ファイル:**
- `base-infra/istio/gateway-*.yaml` (存在する場合)

**変更箇所:**

```yaml
tls:
  certificateRefs:
  - name: wildcard-tls  # ← 自分の証明書Secret名に変更
```

## オプション変更項目

### Sealed Secrets

新しいクラスタでSealed Secretsを使用する場合、既存のSealed Secretsは復号化できません。
以下の手順で再作成してください。

**手順:**

1. 新しいクラスタにSealed Secrets Controllerをデプロイ
2. raw secretを作成（`temp/`ディレクトリ内）
3. `kubeseal`コマンドで新しいSealed Secretを生成
4. 既存のSealed Secret YAMLを置き換え

### StorageClass

**ファイル:**
- `base-infra/local-path-provisioner/local-path-provisioner.yaml`

デフォルトのストレージパスを変更する場合:

```yaml
paths: ["/opt/local-path-provisioner"]  # ← ストレージパスを変更
```

## 変更後の確認

すべての変更を適用した後、以下を確認してください:

```bash
# Kustomizeビルドが正常に動作するか確認
kubectl kustomize base-infra/keycloak/overlays/prod/
kubectl kustomize base-infra/keycloak/overlays/dev/

# ドメイン名が正しく設定されているか確認
grep -r "yu-min3.com" base-infra/

# リポジトリURLが正しく設定されているか確認
grep -r "github.com/yu-min3" base-infra/
```

## トラブルシューティング

### ドメイン名の変更を忘れた場合

Keycloakが起動しても、設定されたホスト名と実際のアクセスURLが一致せず、アクセスできません。

**解決策:** 上記「1. ドメイン名」を参照して修正してください。

### IPアドレス範囲の競合

LoadBalancer IPが割り当てられない、またはネットワーク通信が不安定になります。

**解決策:** DHCPサーバーの設定とCilium LoadBalancer IP Poolを確認してください。
