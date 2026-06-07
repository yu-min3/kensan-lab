---
name: secret-health
description: Secret 管理 4 方式（Vault dynamic / Vault static via ESO / Vault Transit / SealedSecret）の健全性を一括チェック — Vault seal 状態、ESO 同期、SealedSecret 復号、関連 cert をまとめて診断
argument-hint:
---

# Secret Stack Health Check

4 方式が併用されているため（SoT: `docs/secret-management/index.md`）、障害時の切り分けに必要な状態を 1 回で横断確認する。

## Steps

1. **Vault 本体**:
   ```bash
   kubectl get pods -n vault -o wide
   kubectl exec -n vault vault-0 -- vault status
   ```
   - `Sealed: false` を確認。**sealed なら以降の ESO / dynamic creds は全滅する**ので最優先で報告
   - HA 構成でない点に注意（vault-0 の単一障害点）

2. **ClusterSecretStore（ESO ↔ Vault の接続）**:
   ```bash
   kubectl get clustersecretstore -o wide
   ```
   - `vault-backend` が `Valid` / `Ready` であること。ここが死んでいると全 ExternalSecret が巻き添え

3. **ExternalSecret 同期状態（Vault static 方式）**:
   ```bash
   kubectl get externalsecret -A -o custom-columns='NAMESPACE:.metadata.namespace,NAME:.metadata.name,STORE:.spec.secretStoreRef.name,REFRESH:.spec.refreshInterval,STATUS:.status.conditions[0].reason,READY:.status.conditions[0].status'
   ```
   - `SecretSynced` / `True` 以外を列挙。`SecretSyncedError` は Vault path の存在・token 権限を疑う
   - refreshInterval 既定 1h — 直近の Vault 値変更が未反映なだけの場合は経過時間も添える

4. **VaultDynamicSecret / dynamic creds（DB credential 系）**:
   ```bash
   kubectl get vaultdynamicsecret -A 2>/dev/null
   kubectl get vaultauth,vaultconnection -A 2>/dev/null
   ```
   - VSO 系 CR がある場合は同期状態を確認（Keycloak / Polaris の Postgres creds など。ADR-008 参照）

5. **SealedSecret 復号状態**:
   ```bash
   kubectl get sealedsecret -A
   kubectl logs -n sealed-secrets -l app.kubernetes.io/name=sealed-secrets --tail=50 | grep -iE "error|unable" || echo "no decryption errors"
   ```
   - SealedSecret があるのに対応する Secret が生成されていないものを列挙
   - 復号エラーは「別クラスタ / 再 bootstrap 後の鍵不一致」が典型。`/sealed-secret` で再封緘を提案

6. **Secret 消費側の整合**:
   ```bash
   kubectl get pods -A --field-selector=status.phase=Pending -o wide 2>/dev/null
   kubectl get events -A --field-selector=reason=FailedMount --types=Warning 2>/dev/null | head -20
   ```
   - Secret 未生成による `CreateContainerConfigError` / FailedMount を検出

7. **Reloader の生存確認**（rotation の最終工程）:
   ```bash
   kubectl get pods -n reloader
   ```

8. **Summary**:
   - 方式別（Vault dynamic / static / Transit / SealedSecret）に ✅ / ⚠️ / ❌ で 1 行ずつ報告
   - 異常があれば、どの層（Vault 本体 → Store → 同期 → 消費）で切れているかを特定して `/troubleshoot <component>` への導線を示す
   - cert 系の異常が疑われる場合は `/cert-check` を案内（本 skill では深追いしない）

## Notes

- 本 skill は **read-only**。修正操作（unseal、re-seal、secret 再作成）は提案のみ行い、実行はユーザー確認を取る
- Vault unseal が必要な場合: unseal key は Git 管理外。手順はユーザーに委ねる
