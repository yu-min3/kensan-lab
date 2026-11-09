# Runtime OIDC contract

実runtimeのOIDC／HTTP／job台帳を使うconsumer契約試験は、runtime repoに配置する。共有Go moduleとしてconsumerへimportせず、providerのみstubにする。

実装branchはkoubouの`test/llmops-runtime-oidc`。試験ソースは`runtime/oidc_contract_test.go`、実行入口は`scripts/test-runtime-oidc-contract.sh`、詳しい契約は`docs/llmops-oidc-contract.md`。関連draft PRのURLは実装結果へ記録する。

ローカルKeycloak bootstrapで生成したclient directoryと公開CA、同じHTTPS issuerを実行スクリプトへ渡す。runtime向け3clientsと、audience不正・allowlist外・3秒TTLのprobeを分ける。secret値はGitへ保存しない。

2026-11-09に確認した項目:

- 正しい実JWTのCreate／Get、同key同job、同key異内容409
- 別ownerのGet／cancel／retry404、別owner同keyの独立
- malformed JWT、署名改変、audience違い、client allowlist外、実token失効401
- waiting再開と取消、failed retry409
- 実process SIGKILL→同job failed/interrupted復元、復元後のowner拒否

model生成・購読資格・本番接続はこの試験に含まない。通常のunit試験の成功を実OIDC試験の代用にしない。
