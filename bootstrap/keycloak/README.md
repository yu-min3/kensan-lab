# Keycloak Bootstrap (Stage 1)

Vault Stage 1 を起こすために Keycloak 側で必要な OIDC client + user + groups を一括作成するスクリプト。

## なぜ Bootstrap?

Keycloak 自体は GitOps (ArgoCD) で稼働中だが、**realm/groups/user/OIDC client** は GitOps で宣言してない。

選択肢:
- **A**: 全部 GUI で手動 → 再現性ゼロ
- **B**: kcadm.sh をスクリプト化（このアプローチ） → 単一ファイル、再現性 OK、依存なし
- **C**: keycloak-config-cli or Terraform Provider → 宣言的だが依存物多い、Stage 1 にはオーバースペック

Stage 1 の用途（Vault 立ち上げ用 OIDC 1つだけ）には B で十分。
将来 OIDC client が増えてきたら C に移行検討。

## やること

1. realm `kensan` 作成
2. groups `platform-admin`, `platform-dev` 作成
3. user `yu` (email: ymisaki00@gmail.com) を作成 → `platform-admin` に assign
4. OIDC client `vault` を作成（Vault Stage 1 Bootstrap TF が使う）
5. client_secret + user password を Bitwarden に保存

## 前提

- kubectl context が kensan-lab cluster
- Keycloak が `platform-auth-prod` namespace で稼働中
- Bitwarden CLI (`bw`) install + login + unlock 済み
  ```bash
  export BW_SESSION=$(bw unlock --raw)
  ```
- `jq`, `openssl` install 済み

## 実行

```bash
chmod +x bootstrap/keycloak/setup.sh
./bootstrap/keycloak/setup.sh
```

冪等。既存リソースは skip する。

## 出力されるもの

| 場所 | 内容 |
|------|------|
| Bitwarden `kensan-lab/keycloak/oidc-client-vault` | client_id (`vault`) + client_secret |
| Bitwarden `kensan-lab/keycloak/user-yu` | username (`yu`) + password |
| Keycloak realm `kensan` | groups, user, OIDC client が揃った状態 |

## 次のステップ

このスクリプトの後、Vault Stage 1 を立ち上げる:

```bash
# 1. Vault が ArgoCD でデプロイ済み・Pod が Running なことを確認
kubectl -n vault get pod

# 2. Vault 初期化（一度だけ）
kubectl -n vault exec -it vault-0 -- vault operator init \
  -recovery-shares=5 -recovery-threshold=3
# → root token と Recovery Keys が表示される
# → Bitwarden に保存:
#     kensan-lab/vault/root-token
#     kensan-lab/vault/recovery-keys

# 3. Bootstrap TF
cd bootstrap/vault
cp terraform.tfvars.example terraform.tfvars  # 値を埋める
terraform init
terraform apply

# 4. State 廃棄（Pattern A'：bootstrap 後は VCO が引き継ぐ）
rm -f terraform.tfstate*
```

詳細は `bootstrap/vault/README.md` 参照。

## 削除（やり直したいとき）

```bash
# 注意: realm を消すと client_secret も消えるので Bitwarden item の手動削除必須
kubectl -n platform-auth-prod exec -it deployment/keycloak -- \
  /opt/keycloak/bin/kcadm.sh delete realms/kensan

bw delete item $(bw get item kensan-lab/keycloak/oidc-client-vault | jq -r .id)
bw delete item $(bw get item kensan-lab/keycloak/user-yu | jq -r .id)
```
