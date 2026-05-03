# ADR-009: Shared NetworkPolicy for istio-injected namespaces

## Status

Proposed

## Date

2026-05-03

## Context

ADR-004 established the "default-deny + explicit allow" NetworkPolicy pattern for each target namespace. As secrets-management infrastructure (Vault / ESO / VCO) was rolled out in PR #225, NetworkPolicy was authored independently per namespace, which surfaced two problems:

1. **PR #225 had a port omission**: the `allow-istio-gateway-egress` policy in vault namespace only allowed ports 80/443 to istio-system. Port 15012 (istiod xDS over gRPC mTLS) was missing, so the istio-proxy sidecar couldn't pull xDS config from istiod and the Pod hung in Init forever (startup probe failed continuously). ESO and VCO didn't have any istio-system egress policy at all and hit the same wall.

2. **Existing namespaces (`platform-auth-prod` Keycloak, `kensan-prod`, `backstage`, etc.) all use the same `allow-istio` policy**: a single NetworkPolicy that allows all-port bidirectional traffic to/from istio-system namespace. This pattern is already established as best practice in the cluster but wasn't reused in PR #225.

In short, an established pattern existed but was reinvented (poorly) in a new PR. As we add more istio-injected namespaces, the same problem will recur.

## Decision

### Short term (this PR)

Replace the per-namespace hand-written istio policies with the established `allow-istio` pattern:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-istio
spec:
  podSelector: {}
  policyTypes: [Ingress, Egress]
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: istio-system
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: istio-system
```

No port restriction. Bidirectional. The `istio-system` namespaceSelector is the access boundary. This is the same shape as the existing `allow-istio` policies in `infrastructure/security/keycloak/base/network-policy.yaml`, `infrastructure/environments/kensan-{prod,dev}/network-policy.yaml`, etc.

Apply to: `vault`, `external-secrets`, `vault-config-operator`.

### Medium term (separate PR, chore)

Migrate to a single `CiliumClusterwideNetworkPolicy` (CCNP) that applies to any namespace labeled `istio-injection: enabled`:

```yaml
apiVersion: cilium.io/v2
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: allow-istio-for-mesh-ns
spec:
  description: "All namespaces with istio-injection=enabled may freely talk to istio-system"
  endpointSelector:
    matchLabels:
      k8s:io.cilium.k8s.namespace.labels.istio-injection: enabled
  ingress:
    - fromEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: istio-system
  egress:
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: istio-system
```

Place at `infrastructure/network/istio/resources/clusterwide-allow-istio.yaml`. Once verified, remove the per-namespace `allow-istio` from each component's `resources/network-policy.yaml`.

The CCNP supplements the per-namespace `default-deny-all` (Kubernetes NetworkPolicy semantics are additive: any policy that explicitly allows a destination unblocks it). So the chain becomes:

- per-ns `default-deny-all` → blocks everything by default
- CCNP `allow-istio-for-mesh-ns` → adds an istio-system hole to all `istio-injection=enabled` namespaces

### Why CCNP over alternatives

| Alternative | Pros | Cons |
|-----|------|------|
| **CCNP** (chosen) | One file. Namespace label is single source of truth. New ns auto-covered just by adding the label. | Cilium-specific (we're locked to Cilium anyway). Slight learning curve for the namespaceSelector syntax. |
| Kustomize component (`_shared/istio-injected-ns/`) | Stays portable across CNIs. | Each component still has to opt in via `kustomization.yaml`, easy to forget. Bigger blast radius if shared component changes. |
| Kyverno `generate` policy | Auto-generates NP when ns label appears. | New controller dependency. Generated resources are not in Git → ArgoCD `IgnoreExtraneous` workaround needed. Breaks pure GitOps. |
| Shared Helm chart (`platform-namespace`) | GitOps-clean, parameterizable per component. | Extra chart authoring overhead. Each component's Application CR becomes more complex. |

CCNP wins on simplicity (one file, one source of truth) and aligns with how Cilium policies already integrate cleanly with our standard K8s NetworkPolicies.

## Consequences

### Positive

- **One pattern**: any new istio-injected namespace just needs `istio-injection: enabled` label. NP coverage is automatic via CCNP.
- **Smaller surface for mistakes**: PR #225's port-omission bug class becomes structurally impossible.
- **Reduced duplication**: 6+ namespaces currently carry the same hand-written `allow-istio` block; CCNP collapses them.

### Negative

- **Cilium lock-in deepens**: switching CNI requires re-implementing the rule. Acceptable since kensan-lab is committed to Cilium (kube-proxy replacement, L2 announcements).
- **Coarse granularity**: CCNP allows full ns→ns access, not specific ports. For production-grade segmentation, future per-ns policies may need to add more restrictive rules on top. Acceptable for Phase 1; revisit when istio-csr or stricter mTLS enforcement is introduced.

### Migration plan (medium term)

1. Land CCNP `allow-istio-for-mesh-ns` (separate PR).
2. Verify Vault / ESO / VCO / Keycloak / kensan-* / backstage all stay healthy.
3. Remove per-namespace `allow-istio` from each component's `resources/network-policy.yaml` (one PR per component or batch).
4. Update ADR-004 with a note pointing to ADR-009.

## Related

- [ADR-004: NetworkPolicy Design](004-network-policy-design.md)
- [ADR-005: Istio Native OAuth2](005-istio-native-oauth2.md) (Phase 1 sidecar inject decision)
