---
type: note
title: "ArgoCD の prune 保護は個別リソースに付ける"
tags: [argocd, gitops, kubernetes]
status: active
created: 2026-07-06
updated: 2026-07-11
---

## 概要

ArgoCD の `Prune=false` を Application メタデータに付けても、子リソースは守られない。stateful なリソース（PVC / StorageClass / RecurringJob）は、それぞれの manifest 個別に annotation を付ける必要がある。

## ポイント

- Application 側の `Prune=false` は「root-app が Application CR 自体を prune するのを防ぐ」効果しかない
- PVC を Git から消したときに実データごと消える事故は、この誤解が原因
- 守りたいリソースの `metadata.annotations` に直接付ける:

```yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-options: Prune=false
```

## 参考

- 実際に PVC が prune された事例を機に、clusterwide policy と Longhorn の SC / RecurringJob に個別付与した
