# encrypt-users-name

Stage 6 (Vault Transit) Phase 1 用、`users.name` を一括暗号化する 1 度きりの batch CLI。

## いつ走らせるか

```
008_users_name_transit.sql   ← apply (NULL 許容で name_enc / name_hash 追加)
   ↓
このCLIを 1 度走らせる        ← 既存全 users 行で name → name_enc / name_hash
   ↓
user-service Pod を新 image で起動 (PR #2)、動作確認
   ↓
009_drop_users_name.sql       ← apply (旧 name DROP、name_enc / name_hash NOT NULL)
```

順序を逆にすると `column "name_enc" does not exist` (008 未適用) や
`column "name" does not exist` (009 適用後に CLI を回す) で fail する。

## 使い方 (cluster 内)

```bash
# 1. 一時 Pod として実行 (推奨)
#    Vault auth + DB cred + image を bundle するため、user-service と同じ
#    image / SA を使い、entrypoint だけ書き換えて Job として流す。
#    詳細は handoff (Yu に確認)

# 2. ローカル port-forward で実行 (調査・小規模 backfill 用)
kubectl port-forward -n kensan-data svc/postgresql 5432:5432 &
kubectl port-forward -n vault svc/vault 8200:8200 &

DB_HOST=localhost DB_PORT=5432 \
DB_USER=$(kubectl get secret -n kensan-data kensan-db-credentials -o jsonpath='{.data.POSTGRES_USER}' | base64 -d) \
DB_PASSWORD=$(kubectl get secret -n kensan-data kensan-db-credentials -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d) \
DB_NAME=kensan \
VAULT_ADDR=http://localhost:8200 \
VAULT_TOKEN=$(kubectl get secret -n vault vault-root-token -o jsonpath='{.data.token}' | base64 -d) \
VAULT_TRANSIT_KEY=users-name \
  go run ./cmd/encrypt-users-name -dry-run

# OK そうなら -dry-run を外す
go run ./cmd/encrypt-users-name
```

注意:
- `kubectl get secret ... -o jsonpath` は kensan-lab の hook で block される可能性あり。
  実機では Vault に root token を作って手元にだけコピーして実行すること。
- 上記の port-forward 方法は **手元実行**の例。プロダクション運用では Job 化推奨。

## ローカル開発 (Vault 不在)

`VAULT_ADDR` を unset にすれば `vault.NoOpEncryptor` で動く (passthrough、
ciphertext = `noop:<plaintext>`)。dev DB の確認だけしたいときに使う。

```bash
DB_HOST=localhost DB_PORT=5432 DB_USER=kensan DB_PASSWORD=kensan DB_NAME=kensan \
  go run ./cmd/encrypt-users-name -dry-run
```

## フラグ

| フラグ | デフォルト | 説明 |
|---|---|---|
| `-dry-run` | false | 何もしない、ログだけ出す |
| `-force` | false | name_enc が既に non-NULL の行も再 encrypt (key rotation 用) |
| `-batch` | 100 | progress ログを何行ごとに出すか |

## Exit code

- 0: 成功 (or dry-run 完走)
- 1: fatal (DB 接続失敗 / Vault 未起動 等)
- 2: 部分失敗 (一部行で encrypt 失敗、ログに件数あり)

## Key rotation 後の rewrap

key を rotate すると、新規 encrypt は最新 version で行われるが、既存の旧 version
ciphertext は decrypt は可能でも version が古いまま。揃えたいときは:

```bash
# 1. rotate
kubectl exec -n vault vault-0 -c vault -- vault write -f transit/keys/users-name/rotate

# 2. force re-encrypt all rows
go run ./cmd/encrypt-users-name -force
```
