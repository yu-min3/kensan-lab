# NetworkPolicy operations guide

> **SoT for the current design**: the post-CCNP-consolidation network policy design is owned by [ADR-009](../adr/009-shared-allow-istio-network-policy.md) and `kubernetes/network/README.md`. This page is the operations guide (how to write policies, how the machinery works). For the design background, see [ADR-004](../adr/004-network-policy-design.md).

The old hand-written per-ns default-deny — built around the legacy 3-ns layout (`kensan-prod` / `kensan-dev` / `kensan-data`, `platform-auth-dev`) — has been retired. After dropping dev/prod separation and merging `kensan-prod` + `kensan-data` into `kensan` in Phase 3a, the baseline is now consolidated into CiliumClusterwideNetworkPolicy (CCNP).

## Baseline: four CCNPs (shared across mesh namespaces)

Four CCNPs apply across **every namespace** labeled `istio-injection: enabled`. Create a new mesh ns and it is covered automatically, with no extra work (see below).

| CCNP (`kubernetes/network/network-policy/`) | effect |
|---|---|
| `clusterwide-default-deny-for-mesh-ns` | switches ingress + egress to default-deny (selects endpoints with `reserved:none` while allowing nothing) |
| `clusterwide-allow-dns-for-mesh-ns` | opens egress to DNS in kube-system |
| `clusterwide-allow-istio-for-mesh-ns` | opens egress to istio-system (sidecar xDS) |
| `clusterwide-allow-prometheus-scrape-for-mesh-ns` | opens scrape ingress from monitoring |

By Cilium's semantics these are **additive**: default-deny sets the floor, and the allow CCNPs open each direction individually. All four are guarded against pruning per-resource with `argocd.argoproj.io/sync-options: Prune=false`.

Out of scope (no `istio-injection`, so they stay on per-ns NetworkPolicy): `cert-manager`, `cloudflare-tunnel`, `vault`, `external-secrets`, `vault-config-operator`, etc. These get individual NPs in `kubernetes/network/network-policy/<ns>.yaml`.

## Writing per-ns add-on policies

Since the CCNPs lay the DNS / Istio / scrape floor, each ns only needs to allow **the traffic that's still missing** via a per-ns NetworkPolicy. These live in the PE-owned `kubernetes/network/network-policy/<ns>.yaml` (a self-contained app netpol may exceptionally live in the app's own `resources/` — e.g. `app-kensan`'s `syncthing-guard`).

Example — `kubernetes/network/network-policy/kensan.yaml`:

- `allow-intra-namespace` — all pod-to-pod traffic within the ns (microservice ↔ PostgreSQL ↔ MinIO ↔ Polaris ↔ Dagster, all inside the consolidated ns)
- `allow-otel-egress` — egress to monitoring:4318 (OTel Collector)
- `allow-external-ai-egress` — only `app: kensan-ai` pods to external:443 (AI API)
- `allow-vault-egress` — only `app: user-service` pods to vault:8200 (Transit)
- `allow-dagster-external-egress` — only Dagster pods to external:443 (Slack / Gmail / weather API, etc.)

Key points:

- **Scope external egress to specific pods with `podSelector`** (don't open external:443 to the whole ns)
- **Cross-ns traffic needs both sides**: egress on the source ns, ingress on the destination ns
- Don't write DNS / Istio / Prometheus scrape per-ns — the CCNPs already cover them

```yaml
# Example: allow egress to an external API from specific pods only
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-external-api
  namespace: <ns>
spec:
  podSelector:
    matchLabels:
      app: my-service      # scope to target pods
  policyTypes:
    - Egress
  egress:
    - ports:
        - protocol: TCP
          port: 443
```

## How a new namespace gets covered automatically

The CCNP `endpointSelector` matches `k8s:io.cilium.k8s.namespace.labels.istio-injection: enabled`, so **just labeling the namespace `istio-injection: enabled`** instantly applies the default-deny + DNS/Istio/scrape floor. There's no need to write per-ns default-deny / allow-dns / allow-istio / allow-prometheus-scrape.

```bash
# When creating a new mesh ns (adding the label auto-applies the CCNPs)
kubectl label namespace my-new-ns istio-injection=enabled
```

The only thing left to add is "egress specific to that ns" (external APIs, cross-ns DB, etc.).

## Troubleshooting

### When traffic is being blocked

```bash
# List per-ns NetworkPolicies
kubectl get networkpolicy -n <namespace>

# List clusterwide CCNPs
kubectl get ciliumclusterwidenetworkpolicy

# Cilium's applied policy state
kubectl -n kube-system exec -it <cilium-pod> -- cilium policy get

# Watch drops in real time
kubectl -n kube-system exec -it <cilium-pod> -- cilium monitor --type drop

# Inspect flows with Hubble
hubble observe --namespace <namespace> --verdict DROPPED
```

### Common problems

| Symptom | Cause | Fix |
|------|------|------|
| Pods in a new ns can't talk at all | the `istio-injection: enabled` label is missing, so the allow CCNPs aren't taking effect | add the label to the ns |
| Pod can't resolve DNS | a non-mesh ns (no CCNP) without per-ns DNS egress | add DNS egress to the per-ns NP |
| Can't reach an external API | per-ns egress on 443 is missing | add egress with a `podSelector` |
| Cross-ns traffic fails | policy on only one side | add egress on the source and ingress on the destination |

### Emergency: temporarily disabling the baseline

Only when a CCNP is confirmed to be the cause, temporarily remove it (**this ripples across every mesh ns — be careful**):

```bash
kubectl delete ciliumclusterwidenetworkpolicy default-deny-for-mesh-ns
```

This is a stopgap. Once the cause is found, restore it by syncing from Git (note: CCNPs are `Prune=false`, so ArgoCD won't auto-delete them even if they're removed from Git).

## Related

- [ADR-004](../adr/004-network-policy-design.md) — NetworkPolicy design
- [ADR-009](../adr/009-shared-allow-istio-network-policy.md) — shared allow-istio + CCNP consolidation
- `kubernetes/network/README.md` — network directory overview
