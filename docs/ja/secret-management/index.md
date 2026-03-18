# シークレット管理ガイド

このプラットフォームでは、Gitリポジトリに機密情報を安全に保存するため、[Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) を使用します。このガイドでは、各種コンポーネントで必要となるシークレットの作成と暗号化の手順を説明します。

## 前提条件

- `kubeseal` CLIがインストール済みであること。
- Sealed Secretsコントローラーがクラスターにデプロイ済みであること（`infrastructure/sealed-secret/controller.yaml`）。
- `kubectl` がクラスターに接続可能であること。

## Sealed Secretsの仕組み

1.  `kubectl create secret --dry-run=client` で生のSecret（YAML）を生成します。
2.  生のSecretを `kubeseal` CLIにパイプで渡して暗号化します。
3.  暗号化された `SealedSecret` リソース（YAML）が生成されます。
4.  この `SealedSecret` をGitにコミットし、Argo CDでデプロイします。
5.  クラスター上で動作するSealed Secretsコントローラーが `SealedSecret` を復号化し、通常の `Secret` リソースを作成します。

**注意**: 暗号化前の生のSecretは絶対にGitにコミットしないでください。`.gitignore` に `temp/` ディレクトリが含まれていることを確認してください。

---

## 1. GHCR用イメージプルシークレット

GitHub Container Registry (GHCR) からプライベートコンテナイメージをプルするための認証情報です。

以下のスクリプトを実行します。スクリプト内のプレースホルダ (`<github-username>`, `<PAT>` など) をご自身の情報に置き換えてから実行してください。

```bash
./scripts/05-create-ghcr-secret.sh
```

これにより、`infrastructure/sealed-secret/ghcr-pull-secret-prod.yaml` のような暗号化された`SealedSecret`が生成されます。

---

## 2. Grafana管理者パスワード

Prometheusスタックに含まれるGrafanaの管理者パスワードを設定します。

以下のスクリプトを実行すると、ランダムなパスワードが生成され、暗号化された`SealedSecret`が `infrastructure/prometheus/grafana-sealed-secret.yaml` に保存されます。

```bash
./scripts/06a-create-grafana-secret.sh
```

---

## 3. Backstage用シークレット

Backstageが使用するPostgreSQLデータベースの認証情報と、GitHub連携用のPersonal Access Tokenを設定します。

以下のスクリプトを実行します。スクリプト内のプレースホルダ (`<strong-password>`, `<github-pat>`) をご自身の情報に置き換えてから実行してください。

```bash
./scripts/07b-create-backstage-secrets.sh
```

これにより、以下の2つの暗号化済みファイルが生成されます。
- `infrastructure/backstage/postgresql-secret.yaml`
- `infrastructure/backstage/backstage-secret.yaml`
