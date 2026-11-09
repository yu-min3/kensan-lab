# 配備案と検証の境界

このディレクトリは**Argo CDへ未接続の配備案**。本番Application、Namespace、Secret、Polaris／PostgreSQL／S3自体は作成しない。ここを本番探索先へ登録する操作は、draft PRレビュー後の別の配備判断とする。

## 用意したもの

| 構成 | 動作 |
|---|---|
| API Deployment | replicas=1、Recreate。Go APIの排他writerに合わせ、同じPVCを複数writerへ渡さない |
| PVC | Longhorn、RWO、Prune=false。API job台帳を/data/jobsに保存 |
| Secret参照 | 既存Secretだけを参照。values／ConfigMapへ資格の実値を入れない |
| Service | API自身のTLSへ443→8443で接続。APIがJWTを検証し、SSO redirectに依存しない |
| NetworkPolicy | 既定でingressなし、egressはDNSだけ。identity／S3／runtime等への明示許可が必要 |
| worker Job | 既定off。app×role限定Secretと設定を渡す手動実行例。backoff=0でLLM再実行を防ぐ |
| route | 既定off。HTTPS Gateway、HTTPRoute、backend TLS policy／CAを合わせて指定した場合だけrender |

image repository／tag、existingSecret、identity URL、store／runtime設定、client mappingはoperatorの必須入力。latest tagは拒否する。tests/values.jsonは実行先を持たないrender用の一般化した例で、配備値ではない。

## APIと秘密ファイル

Go設定は現行cmd/llmops-apiのJSONと同じ。config.jsonとclient-mapping.jsonだけをConfigMapへ置く。S3資格JSON、runtime client secret、cursor key、TLS cert／key、CAは既存Secretのflat keyとして用意する。apiConfigの各*_fileは/run/privateの参照へ固定する。

既存APIはprivate fileを0400／0600で検査する。K8sのprojected Secretを0440＋fsGroupで非root initへ読ませ、memory emptyDirへ0600でコピーする。APIはそのprivate領域をreadonlyでmountし、projected Secret本体はmountしない。initとAPIはUID/GID 65532、capabilities drop、read-only root、権限昇格禁止。init imageには/bin/sh、cp、basename、chmodを持つ非root対応のimageが必要。

private copyは起動時だけなので、Secret rotation後はsecretRevisionを更新してRecreateする。config／mapping変更はchecksumで再起動する。既存jobのruntime target／clientが消えると再照合できないため、終端まで旧route資格を保持する。

## 配備前に決めること

1. API imageのmulti-arch build／registry、digest、init imageを確認する。DockerfileはAPIだけのbuild定義で、configやcredentialをimageへ入れない。
2. 外部Keycloak／JWKS、runtime、S3のTLS／CAと限定資格を用意する。S3資格はraw、records/feedback、records/job-*等の必要prefixだけを許可し、全Lake共通rootを使わない。
3. ingressで許可するアプリnamespace／podと、egressのidentity／S3／runtime IP・portを特定する。NetworkPolicyのDNS例のlabelも対象clusterで照合する。FQDNルールを標準NetworkPolicyが解決すると仮定しない。
4. route.enabledを使う場合、GatewayがHTTPS listenerと対象namespaceを許可し、BackendTLSPolicyの**実際にインストールされたAPI version**、Serviceの証明書SANに合うbackendHostname、CA ConfigMapを指定する。TLS設定を省略したHTTP backendへ落とさない。
5. PVC／Polaris DB／S3／API台帳／runtime対応を含む復元と、復元後のapp拒否を確認する。PVCを削除して切り戻さない。APIを停止して旧image／設定へ戻し、互換snapshotから復旧する。

healthzはプロセスhealthであり、全依存先の正常・認可成功を保証しない。配備判定にはAPI要求と実サービスintegrationが別途必要。

## Workerの制約

Jobは共通APIのimageでは動かない。worker専用imageにllmops-worker、許可adapter、DuckDB／Python等、/bin/shが必要。configMapはconfig.json、inputConfigMapはrequest.jsonを持ち、worker.argsで実際のsubcommandと設定pathを指定する。入力に秘密を入れない。個人のRawを公開ConfigMapへ置かず、実利用時は認可済みobject参照で渡す。

workerのexistingSecretは**そのapp×role専用**。APIが持つ複数appのSecretを流用しない。runtimeが必要なevalとtable更新を行うtransformを別実行へ分ける。このJobは一回限りの例であり、固定名のまま繰返し更新して再実行する運用は未設計。worker imageの組立・K8sでの実行は未検証。

## オフライン検証

validate.shはHelm 3とPython 3／PyYAMLで、helm lint --strict、template、構造assertを実行する。rendered YAMLはmemory上だけで扱い、commitしない。API単一writer、PVC保護、秘密mount、security context、既定network isolation、route／workerのopt-in、無効なimage／TLS入力の拒否を確認する。

apiserver／CRD admission検証は未実施。helm template --validateはcluster discoveryが必要なので、オフライン検証済みとは表現しない。kubectl apply／helm install／registry pushは実行していない。Dockerfileのcontainer build／起動もこの差分では未検証で、配備前の確認に残す。ローカルGo buildとHTTP動作はAPI実装側の検証を参照する。
