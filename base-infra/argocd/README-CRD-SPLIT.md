# CRD Split

このディレクトリでは、CRDとその他のリソースを分離しています：

- **00-crds.yaml** (1.3MB): 3個のCustomResourceDefinitions
  - Application
  - ApplicationSet
  - AppProject

- **argocd-install.yaml** (102KB): 50個のリソース
  - Deployments, Services, ConfigMaps, Roles, ServiceAccounts等

## デプロイ順序

Argo CDはファイル名のアルファベット順でリソースを適用するため、`00-crds.yaml`が最初に適用されます。

## 再生成方法

Argo CDを更新する場合：

```bash
# 公式マニフェストをダウンロード
kubectl create namespace argocd --dry-run=client -o yaml > namespace.yaml
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml --dry-run=client -o yaml > /tmp/argocd-all.yaml

# CRDとリソースを分離
python3 scripts/split_crds.py /tmp/argocd-all.yaml 00-crds.yaml argocd-install.yaml
```
