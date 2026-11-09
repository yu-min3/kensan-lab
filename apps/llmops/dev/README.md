# ローカル開発環境

Keycloak・Polaris・PostgreSQL・RustFSを独立したcompose project `llmops-dev`で動かす。imageはすべてdigestで固定し、対応する一覧を`versions.lock`に置く。本番realm・クラスタ・バックアップ用bucketには接続しない。

## 起動と停止

repo rootから`apps/llmops/dev/run.sh`を実行する。Docker、Python 3、OpenSSLが必要。初回はimageを取得し、Git管理外の`temp/llmops-dev/`へ以下を生成する。

| ファイル | 用途 |
|---|---|
| `dev.env` | ランダムな検証用資格。0600。初回以後再利用 |
| `ca.pem` / `ca.key` | この環境だけのCA。秘密鍵は0600、公開CAは0644 |
| `server.pem` / `server.key` | id.localhostの30日TLS証明書。秘密鍵は0600 |
| `init.sql` | Polaris schemaとKeycloak専用DB/user。資格を含むため0600 |
| `realm/llmops-realm.json` | llmops realmの初期設定。ユーザー/clientは後続bootstrapで追加 |

`stop.sh`はこのcomposeだけを停止する。volumeと資格は削除しない。再起動してもcatalog metastoreとKeycloak realmは保持する。既存環境とはproject名・volume・portを分離している。

## サービスと境界

| サービス | ローカル接続先 | 永続化 |
|---|---|---|
| Keycloak | `https://id.localhost:19443/realms/llmops` | 専用PostgreSQL database/user |
| Polaris API | `http://127.0.0.1:19381` | PostgreSQLのPolaris schema |
| Polaris health | `http://127.0.0.1:19382/q/health` | なし |
| RustFS | `http://127.0.0.1:19300` | 専用object volume |
| PostgreSQL | compose内部のみ | 専用DB volume |

HTTPのcatalog/object endpointsはローカル開発用。OIDC issuerはhostとcontainerで同じHTTPS URLを使い、ホスト名とCAの両方を検証する。CAをOSのkeychainへ自動登録しない。Keycloakは`cache=local`の単一ノードで動かし、再起動時のdev cluster membership衝突を避ける。HA構成を実証するものではない。

`id.localhost`はMac hostではloopbackへ解決し、compose内部ではKeycloakのnetwork aliasにする。curlは`.localhost`を特別扱いするため、container側のcheckでは`--connect-to`でDocker serviceへ配送する。URLとSNIはid.localhostのまま、TLS検証は有効に保つ。別のHTTP clientを採用するときも、そのclientの名前解決を実接続で確認する。

## 健全性確認

`check.py`は次をassertする。

- hostから明示したCAでHTTPS discoveryを取得できる
- CAを指定しない場合は拒否される
- JWKSが取得できる
- containerから同じHTTPS issuerを検証できる
- catalogとobject storageのhealthが200になる

`run.sh`は起動後にこの検査を実行し、到達・信頼の不足があれば非0で終了する。起動／停止／再起動をMac arm64とDocker Desktopで確認した。

## 資格を混ぜない

RustFS root資格はストレージ自身と後続の管理bootstrapだけが使う。PolarisサーバーやAPI/workerへ同じキーを渡さない。Polarisには将来の限定資格`CATALOG_S3_KEY`／`CATALOG_S3_SECRET`を注入する入口だけを置き、未設定時は空で起動する。catalog作成・app別client・role・S3 policyは[bootstrap](bootstrap/README.md)で構築する。この起動成功だけをapp別認可成功と扱わない。

PostgreSQLの初期化とPolaris metastoreの初期化は異なる。schemaを作成してから、versionの一致した`polaris-admin-tool`でrealmのbootstrapを行う。bootstrap用principal資格は長時間動くPolarisサーバーの環境へ渡さない。

Keycloakのlocal admin資格はtempにのみ生成し、この環境の初期構築に使う。実アプリは専用clientを使い、admin tokenを日常処理へ配らない。

## 次に検証すること

サービス起動とTLS trustに続き、[bootstrap](bootstrap/README.md)がKeycloakのclient credentials/audience、Polarisへの外部認証、限定S3資格、appごとのgrantを構築・検証する。Java/Goの各clientも明示的にこのCAを信頼する設定が必要。

CAとserver証明書は30日で期限切れになる。期限切れ時は接続を失敗させる。共有環境や本番でこれらを流用しない。volumeと資格は対で保持し、volumeを残したままdev.envだけ再生成して異なるDB資格へ置き換えない。

## 一次資料

- [Keycloak TLS](https://www.keycloak.org/server/enabletls)
- [Keycloak hostname](https://www.keycloak.org/server/hostname)
- [Polaris JDBC](https://polaris.apache.org/guides/jdbc/)
- [Polaris RustFS](https://polaris.apache.org/guides/rustfs/)
