# kensan dev 環境のクリーンアップ

dev/prod 分離廃止に伴って、kensan アプリの dev 環境一式を削除する手順。一度きりの手動オペ。

## 対象

- ArgoCD Application: `kensan-app-dev`, `env-kensan-dev` (manifest 削除で自動 prune される)
- Namespace: `kensan-dev`
- ApplicationSet `environments` 経由で生成されていた env-kensan-dev も自動的に消える (`infrastructure/kensan/kensan-dev/config.json` 削除済)
- NetworkPolicy: `infrastructure/network/network-policy/kensan-dev.yaml` (manifest 側で削除済)

## 手順

### 1. ArgoCD Application の自動 prune 確認

```bash
kubectl get application -n argocd -o name | grep kensan-dev
# 想定: NotFound (kensan-app-dev, env-kensan-dev 共)
```

### 2. Namespace の削除

```bash
kubectl delete namespace kensan-dev
```

PVC が finalizer で引っかかる場合は ExternalSecret / StatefulSet を強制削除してから ns 削除。

### 3. PVC データの cleanup (任意)

local-path-provisioner の物理ファイルは残る場合あり:

```bash
# 該当ノードで
sudo ls /opt/local-path-provisioner/ | grep kensan-dev
sudo rm -rf /opt/local-path-provisioner/<該当ディレクトリ>
```

### 4. Vault role の確認

`kensan-users-transit` Vault role の targetNamespaces から `kensan-dev` は manifest で削除済。VCO が自動的に Vault role を更新する。

```bash
# Vault 側で確認
vault read auth/kubernetes/role/kensan-users-transit
# 想定: bound_service_account_namespaces に kensan-dev は含まれない
```

### 5. Image registry の cleanup (任意)

GHCR にまだ dev タグが残っている場合は手動削除。本 PR の範囲外。

## 検証

- `kubectl get ns | grep kensan` → `kensan-prod`, `kensan-data` のみ表示
- `kensan.app.yu-min3.com` が prod に向いて動作 (Pod 起動 / DB 接続 / OIDC ログイン)
