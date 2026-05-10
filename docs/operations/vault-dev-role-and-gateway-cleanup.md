# Vault dev role + gateway-dev のクリーンアップ

dev/prod 分離廃止に伴って、以下を削除する手動オペ。一度きり。

## 対象

- Vault Identity Group `platform-dev` と Group Alias (manifest 削除済 → VCO が自動で Vault 側を削除)
- Vault OIDC role の `platform-dev` boundClaim 削除 (manifest 編集済)
- Istio Gateway `gateway-dev` (manifest 削除済 → ArgoCD が自動 prune)
- Cilium LoadBalancer IP 192.168.0.241 (gateway-dev に割り当てられていた、自動解放)

## 前提

- 当該 PR がマージされ、ArgoCD が新構成に同期済み
- PR 3 (kensan dev 廃止) が先にマージされていること (kensan は既に gateway-prod を参照)

## 影響

- **Keycloak の `platform-dev` グループ所属ユーザー**: Vault にログインできなくなる (`bound_claims` 違反で reject)
  - Yu のみの homelab では実質影響なし (admin が常用)
  - ArgoCD / Grafana の `platform-dev` 権限 (read-only) は継続 (今 PR では触らない)
- **gateway-dev に紐付いていた HTTPRoute**: 該当する HTTPRoute は無いはず (kensan は PR 3 で gateway-prod に移行済み)
  - Backstage scaffolder template は依然 `gateway-dev` を参照しているが、新規 app を立てる時点で問題が出る (別 PR で対応)

## 手順

### 1. ArgoCD の同期確認

```bash
# Vault 関連の Application が同期済みか確認
kubectl get application -n argocd vault-config-operator -o yaml | grep -E "(status|sync|health)" | head -20
kubectl get application -n argocd istio-resources -o yaml | grep -E "(status|sync|health)" | head -20
```

### 2. Vault 側のクリーンアップ確認

VCO が manifest 削除を検知して Vault 側のリソースを削除しているはず。

```bash
# Vault Identity Group (platform-dev) が消えていること
vault read identity/group/name/platform-dev
# 想定: No value found (削除済)

# OIDC role の bound_claims から platform-dev が消えていること
vault read auth/oidc/role/default
# bound_claims.groups = [platform-admin] のみ
```

もし VCO が処理し切れていない場合、手動で削除:

```bash
vault delete identity/group/name/platform-dev
```

### 3. Cilium LoadBalancer IP の解放確認

```bash
kubectl get svc -n istio-system -o wide | grep gateway
# 想定: gateway-prod (192.168.0.243) と gateway-platform (192.168.0.242) のみ
# 192.168.0.241 はどの Service にも紐付かない (再利用可能)
```

### 4. Backstage scaffolder template の TODO

Backstage scaffolder (`backstage/app/templates/fastapi-template/skeleton/`) は依然として `gateway-dev` 参照を含む overlay を生成する。新規 app をスキャフォールドする際は手動で `gateway-prod` に書き換える必要がある。

Phase 4 (カテゴリ README) もしくは別 PR で Backstage scaffolder を整理する想定。

## 検証

- Vault UI / CLI で platform-admin ユーザーがログインできる
- platform-dev のみ持つユーザー (居れば) がログインできない (`bound_claims` 違反)
- ArgoCD で gateway-dev Application が prune されている (Application としては存在しない、Gateway リソースが ArgoCD `directory` source 経由で消える)
- `*.app.yu-min3.com` の HTTPRoute が gateway-prod にバインドされて疎通している (kensan)
