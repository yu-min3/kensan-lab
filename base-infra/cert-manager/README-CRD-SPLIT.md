# CRD Split

このディレクトリでは、CRDとその他のリソースを分離しています：

- **00-crds.yaml** (931KB): 6個のCustomResourceDefinitions
  - CertificateRequest
  - Certificate
  - Challenge
  - ClusterIssuer
  - Issuer
  - Order

- **cert-manager.yaml** (38KB): 43個のリソース
  - Deployments, Services, ConfigMaps, Roles, ServiceAccounts, Webhooks等

## デプロイ順序

Argo CDはファイル名のアルファベット順でリソースを適用するため、`00-crds.yaml`が最初に適用されます。
`ServerSideApply=true`オプションにより、CRDの適用が確実に行われます。

## 再生成方法

Cert-Managerを更新する場合：

```bash
# 公式マニフェストをダウンロード
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml --dry-run=client -o yaml > /tmp/cert-manager-all.yaml

# CRDとリソースを分離
python3 scripts/split_crds.py /tmp/cert-manager-all.yaml 00-crds.yaml cert-manager.yaml
```
