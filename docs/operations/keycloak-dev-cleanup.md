# Keycloak dev 環境のクリーンアップ

dev/prod 分離廃止に伴って、Keycloak の dev realm と関連リソースを削除する手順。一度きりの手動オペ。

## 対象

- Keycloak realm: `kensan-dev` (dev 用 realm)
- ArgoCD Application: `keycloak-dev` (manifest 削除で自動 prune される)
- Namespace: `platform-auth-dev`
- Vault dynamic DB credential role: `keycloak-dev`
- NetworkPolicy: `infrastructure/network/network-policy/keycloak-dev.yaml` (manifest 側で削除済)
- Vault DB engine config: `infrastructure/security/vault-database-engine/platform-values/vault-database/keycloak-dev.yaml` (manifest 側で削除済)

## 前提

- 当該 PR がマージされ、ArgoCD が新構成 (single Keycloak in `platform-auth-prod`) に同期済み
- prod realm (`kensan`) は稼働を継続している (本オペで触らない)

## 手順

### 1. 影響範囲の確認

dev realm に依存する OIDC client が無いことを確認:

```bash
# Keycloak admin UI (auth-dev.platform.yu-min3.com) で確認
# - kensan-dev realm の Clients 一覧
# - 想定: 過去のテスト用クライアントのみ
```

### 2. Keycloak realm の削除

```bash
# Keycloak admin UI にログイン (kensan-dev realm)
# Realm Settings → Action → Delete realm
```

または kcadm.sh で:

```bash
kubectl exec -n platform-auth-dev deploy/keycloak -- /opt/keycloak/bin/kcadm.sh \
  config credentials --server http://localhost:8080 --realm master --user admin
kubectl exec -n platform-auth-dev deploy/keycloak -- /opt/keycloak/bin/kcadm.sh \
  delete realms/kensan-dev
```

### 3. ArgoCD Application の自動 prune 確認

`keycloak-dev` Application は manifest 側 (`infrastructure/gitops/argocd/applications/security/keycloak/overlays/dev/`) を削除済なので、ArgoCD が自動的に prune する。

```bash
kubectl get application -n argocd keycloak-dev
# 想定: NotFound
```

### 4. Namespace の削除

ArgoCD prune 後も namespace 自体は残る場合がある (`CreateNamespace=true` で作成された ns は managed-by ラベルが付くが prune 対象外)。手動削除:

```bash
kubectl delete namespace platform-auth-dev
```

ns 削除に時間がかかる場合 (finalizer が引っかかる) は ExternalSecret / Pod を先に強制削除する。

### 5. Vault dev role の削除

PR 4 (Vault OIDC dev role 削除) で `group-alias-platform-dev.yaml` / `group-platform-dev.yaml` が manifest から消える。Vault Config Operator が自動的に Vault 側のリソースを削除する。

dynamic DB credential role (Vault 側 `database/roles/keycloak-dev`) も VCO 管理なので、VDBE manifest 削除で自動的に消える想定。

```bash
# Vault 側で直接確認したい場合
vault list database/roles/
# 想定: keycloak-dev は無い (keycloak-prod のみ)
```

### 6. PVC の cleanup (任意)

local-path-provisioner の PVC は ns 削除と同時に解放されるが、disk 上の `/opt/local-path-provisioner/<pvc-id>` は手動で消す必要がある場合あり:

```bash
# 該当ノード (worker1/2 等) で
sudo ls /opt/local-path-provisioner/ | grep platform-auth-dev
sudo rm -rf /opt/local-path-provisioner/<該当ディレクトリ>
```

## 検証

- `kubectl get ns | grep platform-auth` → `platform-auth-prod` のみ表示されること
- `auth.platform.yu-min3.com` からの OIDC ログインが prod realm で動作していること (ArgoCD / Vault / Grafana 等)
