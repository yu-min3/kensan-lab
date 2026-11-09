# Iceberg compatibility probe

実際のRustFS・Polaris・PostgreSQLとDuckDBを使い、テーブル更新、snapshot固定、認可、再起動を検証する。アプリの業務データや稼働中クラスタは使わない。

## 実行

DockerとPython 3があるMacから`apps/llmops/dev/probes/run.sh`を実行する。最初に依存を取得し、repo内のGit管理外`temp/llmops-probe/`へランダムな検証用資格とvenvを作る。root資格はcatalog/bucket/roleの検証用bootstrapに限定し、読取試験は専用principal・vended S3資格へ切り替える。

公開ポートはloopbackの19200(S3)、19281(catalog)、19282(health)。検証データは毎回新しいランダム名のbucket/catalogへ書く。Polarisだけを再起動し、PostgreSQLとobjectを残して権限の保持を確認する。OAuthの有効期限を検証用20秒に短縮し、期限切れ401とDuckDBの再取得を確認するため、実行には最低20秒かかる。

終了時は`apps/llmops/dev/probes/stop.sh`でこのcomposeのみ停止できる。volumeと資格は保持する。検証に失敗した場合はassertionで非0終了する。root資格を通常アプリへ渡して失敗を回避しない。

## 固定した構成

| 部品 | version |
|---|---|
| RustFS | 1.0.0-beta.8、composeにimage digest固定 |
| Polaris / admin tool | 1.8.0、composeにimage digest固定 |
| PostgreSQL | 18.6、composeにimage digest固定 |
| Python DuckDB | 2.0.0、全Python依存はrequirements固定 |
| DuckDB extension | iceberg `4c0c0874c4`、httpfs `77392a7207`、avro `345c3f4e82`。起動時に一致をassert |

Apache公式RustFS/JDBCガイドを参考にした独自の隔離構成。Postgresのschema作成に加えてpolaris-admin-tool bootstrapが必要。Polaris自身のbootstrap環境変数だけではJDBC metastoreを初期化できなかった。

## 検証すること

- REST catalogでCREATE/INSERT/SELECT、AからBへ更新後のA固定読取
- 読取専用principalによるSELECT成功とINSERTの認可拒否
- 別namespace/tableの読取拒否
- table-scoped S3 sessionによる別prefix GET拒否、同table PUT拒否
- 期限切れcatalog bearer拒否と、同じDuckDB OAuth secretによる再取得
- Polaris再起動後のtable・snapshot・principal/grant保持

## このprobeで保証しないこと

Keycloak OIDC・HTTPS issuer・local CAは次の開発環境構築で検証する。ここではPolaris内蔵OAuthを使う。Raw/run領域に対するapp別資格、S3 session自体の満了後更新、長時間query、backupからの復元は別の統合試験にする。catalogのtoken再取得とS3 sessionの期限は別物である。NAMESPACE_LISTは試験catalog全体へ付与しているためnamespace名の秘匿も保証しない。

DuckDB extensionは取得後のrevisionを照合し、変わった場合は成功扱いにせず再検証を要求する。今回の証拠はarm64 Mac clientとDocker Desktopで取得し、Linux/amd64の動作を代用しない。

## 根拠

- [Polaris RustFS guide](https://polaris.apache.org/guides/rustfs/)
- [Polaris JDBC guide](https://polaris.apache.org/guides/jdbc/)
- [DuckDB catalog configuration](https://duckdb.org/docs/2.0/core_extensions/iceberg/catalogs)
