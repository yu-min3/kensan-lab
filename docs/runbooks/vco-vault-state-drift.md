# VCO drift: Vault 再構築後に CR が再適用されない

## 症状

Vault Config Operator の CR（Policy / JWTOIDCAuthEngineRole 等）が `ReconcileSuccessful: True` を示しているのに、**Vault 側に対応するオブジェクトが存在しない**。

実例（2026-07-06 発見）: `JWTOIDCAuthEngineRole/default` が Successful 表示のまま、`vault list auth/oidc/role` に `default` が無い → Vault UI の Role 空欄ログインが「claim "groups" does not match」で失敗。

## 原因

VCO は **generation ベースの reconcile**。CR の spec が変わらない限り再適用せず、**Vault 側の状態消失（再 init・災害復旧での再構築）を検知しない**。`observedGeneration == generation` なら「完了」のまま。

- annotation の付与では発火しない（metadata 変更は generation を上げない）

## 復旧手順

```bash
# 1. 症状確認: CR は Successful だが Vault に実体が無い
kubectl get jwtoidcauthenginerole <name> -n vault -o jsonpath='{.status.conditions}'
# (Vault 側の確認は vault ns の default SA で k8s auth login → vault list)

# 2. CR を削除 → ArgoCD selfHeal が再作成 → 新規 CR として fresh reconcile が走る
kubectl delete jwtoidcauthenginerole <name> -n vault
# (Vault 側に実体が無いので deletion finalizer は no-op。ArgoCD が数十秒で再作成)

# 3. 実体の再作成を確認
kubectl get jwtoidcauthenginerole <name> -n vault \
  -o jsonpath='{.status.conditions[?(@.type=="ReconcileSuccessful")].status}'
```

## 予防・注意

- **Vault を再構築したら、全 VCO CR を delete → selfHeal 再作成で総なめする**（disaster recovery 手順に追加すべき項目）
- `auth/oidc/config`（default_role 等）は VCO 管理外（TF/手動）。partial update 不可のため、
  変更には client_secret 込みの full-write が必要（PR #298 / `temp/switch-vault-default-role.sh` 参照）
