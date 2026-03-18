# CLAUDE.md

## Repository Overview

**kensan-lab** — Kubernetes GitOps platform for bare-metal multi-arch cluster. Manages cluster infrastructure, security, and Argo CD control structures. Owned by Platform Engineers (PE); enables Application Developers (AD) to deploy via Backstage.

## Technology Stack

| Layer | Technology |
|-------|-----------|
| Kubernetes | Bare-metal kubeadm (RPi 5 ARM64 x3 + Bosgame M4 Neo AMD64 x1) |
| Runtime | CRI-O (cluster), **Podman** (image builds — never Docker) |
| CNI / LB | Cilium (kube-proxy replacement, L2 LoadBalancer) |
| Service Mesh | Istio + Gateway API |
| GitOps | Argo CD — Helm multi-source Application pattern |
| Secrets | Sealed Secrets (kubeseal) |
| Certificates | cert-manager + Let's Encrypt |
| Auth | Keycloak (JWT, prod/dev) |
| Observability | Prometheus, Grafana, Loki, Tempo, OTel Collector |
| Developer Portal | Backstage with scaffolder templates |

## Repository Structure

```
infrastructure/                    # Core infra (GitOps-managed)
├── gitops/argocd/                # Argo CD: applications/, projects/, root-apps/
├── observability/                # grafana, prometheus, loki, tempo, otel-collector
├── network/                      # cilium, istio, gateway-api
├── security/                     # cert-manager, sealed-secrets, keycloak
├── environments/                 # app-dev, app-prod, observability, system-infra
└── storage/                      # local-path-provisioner
backstage/                        # Developer portal (app/ + manifests/)
apps/                             # Sample applications
docs/                             # ADRs, architecture, bootstrapping guides
scripts/                          # Automation scripts
temp/                             # Temporary files (git-ignored, raw secrets)
```

## Helm Multi-Source Pattern

Each component = **Application CR** + **values.yaml** + **resources/**. See `.claude/rules/helm-multisource.md` for details.

- **Change config**: Edit `values.yaml` → commit → push
- **Upgrade chart**: Update `targetRevision` in Application CR → commit → push
- **Add resources**: Place YAML in `resources/` → commit → push

## Quick Reference

```bash
kubectl get nodes                          # Cluster health
kubectl get applications -n argocd         # GitOps status
kubectl get gateway -A                     # Gateways
kubectl get certificate -A                 # Certificates
kubectl get pods -A --field-selector=status.phase!=Running,status.phase!=Succeeded  # Unhealthy pods
```

Backstage development:
```bash
cd backstage/app && make install           # Dependencies
cd backstage/app && make dev               # Local dev (localhost:3000)
cd backstage/app && make all TAG=v1.0.0    # Build + push (Podman)
```

## Mandatory Constraints

1. **GitOps only**: ALL infrastructure changes via Git → Argo CD. No direct `kubectl apply`.
2. **Podman, not Docker**: All image builds use Podman.
3. **No rendered manifests**: Argo CD renders Helm charts natively. Never commit `helm template` output.
4. **Secrets via Sealed Secrets**: Raw secrets in `temp/` only. Commit only sealed YAMLs.
5. **No .env commits**: Sensitive tokens stay out of Git.

## Skills (Slash Commands)

| Command | Purpose |
|---------|---------|
| `/sealed-secret <name> <ns>` | Create and seal a new secret |
| `/helm-upgrade <component> <ver>` | Upgrade Helm chart version |
| `/new-component <category> <name>` | Scaffold new infra component |
| `/cluster-status` | Quick cluster health check |
| `/troubleshoot <component>` | Diagnose component issues |
| `/cert-check` | Certificate health & expiry check |
| `/argocd-sync [app]` | Check sync status & drift |

## Domain Rules (`.claude/rules/`)

| Rule File | Scope |
|-----------|-------|
| `gitops-workflow.md` | GitOps principles, deploy order, Podman |
| `helm-multisource.md` | 3-file pattern details |
| `kubernetes-cluster.md` | Node topology, scheduling, storage |
| `network-ingress.md` | Cilium, Gateways, domains, certs |
| `security-secrets.md` | Sealed Secrets, cert-manager, GHCR |
| `environment-separation.md` | PE/AD roles, multi-repo, namespaces |

## User Preferences

- **コマンド出力**: シェルコマンドを提示する際は `temp/` にスクリプトファイル（`.sh`）として書き出す。改行崩れ防止。
- **言語**: 日本語での対話を優先。
- **コミットメッセージ**: 1行目は50文字以内で簡潔に。本文は不要（diffを見ればわかる）。

## Domain & Network

- **ドメイン**: `yu-min3.com`（Cloudflare DNS）
- **LB IP range**: `192.168.0.240-249`
- **GitHub org**: `yu-min3`
- フォークする場合は `docs/configuration.md` を参照してドメイン等を置換すること

## Documentation

- Architecture & ADRs: `docs/`
- Bootstrapping: `docs/bootstrapping/`
- Kustomize guidelines: `docs/kustomize-guidelines.md`
