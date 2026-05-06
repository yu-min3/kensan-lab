# ArgoCD repo-server probe timeout tuning

## 症状

Pi 5 worker が NotReady から復帰した直後に ArgoCD repo-server が CrashLoopBackOff に入る。chart render burst が瞬間的に走ることで livenessProbe / readinessProbe の `timeoutSeconds: 1` (chart default) が短すぎて probe 失敗扱いになる。

## 原因

repo-server は Helm template / git clone を担当しており、reschedule 時には複数 Application の chart render が短時間に集中する。Pi 5 群の CPU 容量に対して default 1 秒の probe timeout が厳しい。

## 設定 (現状)

`infrastructure/gitops/argocd/values.yaml`:

```yaml
repoServer:
  livenessProbe:
    timeoutSeconds: 5
  readinessProbe:
    timeoutSeconds: 5
```

5 秒に伸ばすと負荷スパイク中も probe が通る。通常時の検知遅延は数秒延びるだけなので運用影響は軽微。

## 関連設定

`controller.diff.server.side: "true"`: K8s API server に diff を計算させて field manager 情報を尊重する。K8s API / admission webhook が注入する default 値 (StatefulSet `dnsPolicy`、HTTPRoute `backendRefs[].kind`、ESO `deletionPolicy` 等) を drift と誤検知しなくなる。

ESO admission webhook の default 値注入を drift と誤検知させないため、以下を ignoreDifferences で除外している:

```yaml
resource.customizations.ignoreDifferences.external-secrets.io_ExternalSecret: |
  jqPathExpressions:
    - .spec.target.deletionPolicy
    - .spec.data[].remoteRef.conversionStrategy
    - .spec.data[].remoteRef.decodingStrategy
    - .spec.data[].remoteRef.metadataPolicy
    - .spec.data[].remoteRef.nullBytePolicy
```

## 関連

- [ArgoCD Keycloak integration](../auth/argocd-keycloak-integration.md)
