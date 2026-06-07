# app-base

kensan-lab platform が application に提供する汎用デプロイ chart。
「**PE が chart を提供し、app は values だけで利用する**」パターン（`vault-database-engine` と同型）を app 向けに一般化したもの。app は repo にマニフェストを抱えず、image とドメインなど必要な値だけを渡す。

## 目的

この chart が生成するのは `Deployment` / `Service` / `ServiceAccount` と、任意で `HTTPRoute`（+ oauth2 用）/ `PVC` / `ghcr pull secret`（ExternalSecret）/ OTel 配線。最大の役割は **secure default の強制**で、app 側で緩められない既定を焼き込んでいる:

- `runAsNonRoot: true`、`allowPrivilegeEscalation: false`
- `readOnlyRootFilesystem: true`
- `capabilities.drop: ["ALL"]`
- `seccompProfile.type: RuntimeDefault`（pod / container 両方）
- Pod レベルの `fsGroup: 65532`（PVC を nonroot コンテナが書けるように）

これらは PSA `restricted` 準拠であり、緩めたくなったら values で上書きするのではなく Kyverno exception の議論に回す方針（`values.yaml` のコメント参照）。

## 使い方

ArgoCD の **multi-source** で参照する。chart 本体（このディレクトリ）を source 1、app 固有の `values.yaml` を `$values` ref で渡す。実例は `kubernetes/apps/app-kensan/app.yaml`（CR）と `kubernetes/apps/app-kensan/values.yaml`（values）— **app-base の最初の消費者**。

```yaml
sources:
  - repoURL: https://github.com/yu-min3/kensan-lab
    targetRevision: main
    path: charts/app-base
    helm:
      releaseName: app-<name>
      valueFiles:
        - $values/kubernetes/apps/app-<name>/values.yaml
  - repoURL: https://github.com/yu-min3/kensan-lab
    targetRevision: main
    ref: values
```

namespace 等の raw マニフェストが必要なら 3 つ目の source（`directory.recurse`）として足す。

## 主要 values

全項目と既定値は [`values.yaml`](./values.yaml) を参照。よく上書きするもの:

| key | 既定 | 説明 |
|-----|------|------|
| `image.repository` / `image.tag` | （必須） | 未指定だと render 時にエラー |
| `nameOverride` | Release 名 | リソース名の上書き |
| `env` | `[]` | `[{name, value}]` でそのまま container env に入る |
| `resources` | `{}` | requests / limits |
| `pvc.enabled` / `pvc.create` / `pvc.name` | `false` / `true` / `<release>-data` | `create: false` + `name` で既存 claim に乗る |
| `otel.enabled` | `false` | OTLP endpoint / service name を env で配線（計装自体は app の責任） |
| `httproute.enabled` / `httproute.hostnames` | `false` / `[]` | Gateway API HTTPRoute（gateway: `gateway-prod` / `istio-system`） |
| `auth.gatewayOAuth2.enabled` | `false` | gateway 層 Keycloak SSO（ext_authz）。有効化には PE 側 3 点セットが別途必要（`values.yaml` 参照） |
| `ghcrPullSecret.enabled` | `false` | ns に ghcr pull secret を ExternalSecret で生成し SA に配線 |
| `reloader` | `true` | Secret/ConfigMap 更新時の自動 rollout（stakater/reloader） |
| `nodeSelector` / `affinity` | `{}` | スケジューリング制約（例: amd64 限定） |
