# argocd

Argo CD 自身の管理。コントロールプレーン (Argo CD server / repo-server / application-controller 等) と、それが管理する全 Application / ApplicationSet / AppProject の宣言が集約されている。

## 構成

| dir / file | 役割 |
|---|---|
| `values.yaml` | Argo CD chart の override (HA、Keycloak OIDC、Helm multi-source enable 等) |
| `resources/` | Argo CD ns、`app-project` AppProject、HTTPRoute、OIDC ExternalSecret 等の chart 外マニフェスト |
| `projects/platform-project.yaml` | infrastructure 用 AppProject |
| `root-apps/platform-root-app.yaml` | App-of-Apps エントリ。`applications/` を再帰スキャン |
| `applications/` | 各 component の Application / ApplicationSet CR。`applications/README.md` 参照 |

## App-of-Apps 構造

```
platform-root-app (root)
  └─ scans applications/ recursively
       ├─ applications/network/ (cilium, istio-*, gateway-api, ...)
       ├─ applications/secrets/ (vault, external-secrets, ...)
       ├─ applications/auth/ (keycloak, oauth2-proxy, ...)
       ├─ applications/observability/applicationset.yaml
       ├─ applications/namespaces/ (ns-lifecycle app 群 + app-prod)
       ├─ applications/storage/ (longhorn)
       ├─ applications/backstage/
       └─ applications/apps/ (Backstage 経由で auto-commit される user app)
```

## 関連

- [ApplicationSet 移行方針 (ADR-003)](../../docs/adr/003-applicationset-migration-strategy.md)
- [Helm multi-source 配置規約](../README.md)
- [GitOps workflow](../../.claude/rules/gitops-workflow.md)
