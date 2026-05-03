# ADR-007: Do Not Adopt Vault PKI (Phase 1-3)

## Status

Accepted

## Date

2026-05-03

## Context

When introducing Vault in `secrets-plan.md` Phase 2, we initially considered enabling the Vault PKI engine to issue "internal ingress certs". However, after organizing the kensan-lab TLS posture, we found that there is essentially no operational room for Vault PKI today.

### Current TLS Ownership

| Communication path | Current TLS owner | Vault PKI room |
|---|---|---|
| External user -> Istio Gateway | **Let's Encrypt** (`wildcard-platform-tls` / `wildcard-apps-tls`) | x Browser-untrusted |
| Gateway -> Pod (in-mesh) | **Istio automatic mTLS** (istiod is the CA, 24h auto-rotate) | x Duplicative |
| Pod <-> Pod (both in-mesh) | **Istio automatic mTLS** | x Duplicative |
| Pod -> Postgres (both in-mesh) | **Istio automatic mTLS** | x Duplicative |
| Pod -> out-of-mesh service | App implementation | ~ Only if needed |
| kube-apiserver / etcd | Managed by kubeadm | x |

=> Istio fully automates intra-cluster mTLS via `PeerAuthentication: STRICT`, and there is no gap left for Vault PKI to fill.

### Requirements

1. **Do not break the existing TLS posture**: Istio's built-in CA + Let's Encrypt are running stably in production
2. **Minimize Vault adoption operational cost**: Enabling the PKI engine adds operations for root CA / intermediate CA / role definitions / cert-manager Vault Issuer
3. **Keep future options open**: Preserve a path to adoption if requirements such as audit aggregation or centralized cert revocation arise

### Patterns Considered

#### Pattern A: Adopt Vault PKI in Phase 2

Issue internal ingress certs from Vault PKI via cert-manager's Vault Issuer.

**Pros:**
- All cert issuance is centralized in Vault (audit logs are also centralized)
- Cert revocation can be managed centrally in Vault

**Cons:**
- There is no actual demand for internal ingress certs (Istio mTLS already covers this)
- Requires designing root CA / intermediate CA / roles / TTLs for Vault PKI
- A Vault PKI outage halts cert renewals

#### Pattern B: Use istio-csr to Make Vault the Istio Root CA (Future Option)

Have Istio's workload certs (24h SPIFFE certs) signed by Vault PKI rather than the istiod built-in CA.

**Pros:**
- All certs (including workload certs) flow through Vault
- High completeness as a single-CA reference architecture

**Cons:**
- Istio startup depends on cert-manager + Vault, binding the entire mesh's health to Vault
- The standard istiod built-in CA is fully automated with zero-ops; opting out of it has cost
- istio-csr has few production examples, and operational know-how for a single-operator homelab is thin

#### Pattern C: Do Not Adopt Vault PKI (Adopted)

Maintain the current Istio automatic mTLS + Let's Encrypt posture. Vault is operated only with KV / Database engine / Transit.

**Pros:**
- The TLS posture is already complete; nothing more needs to be put on Vault
- A Vault outage does not halt cert renewals (certs are managed independently by cert-manager + Let's Encrypt)
- No additional operational surface (fits a single-operator homelab)

**Cons:**
- Cannot tell a "centrally managed certs" reference architecture story
- If audit/compliance requirements arrive later, an adoption effort is incurred

## Decision

**Adopt Pattern C. Do not enable the Vault PKI engine in Phase 1-3.**

### 1. Position as the Cert-Side Counterpart of "Do Not Move Everything to Vault"

As a counterpart to ADR-008 (do not move Keycloak DB credentials to Vault), kensan-lab does **not treat Vault centralization as a goal in itself**. We move things to Vault where Vault provides unique value (dynamic credentials / Transit), and do not move things where existing tools already complete the job (certs / Keycloak DB credentials).

### 2. "Internal Ingress Certs" Are Reconsidered Only When Demand Arises

The phrase "issue internal ingress certs from Vault PKI" in `secrets-plan.md` Phase 2 is retracted by this ADR. When concrete internal ingress demand arises, re-evaluate whether Istio mTLS suffices or a separate cert is required.

### 3. Triggers for Future Vault PKI Adoption (Explicit)

If any of the following occurs, update this ADR and reconsider Vault PKI adoption:

- Audit / compliance requirements demand centralized cert issuance
- Centralized cert revocation becomes required (e.g., immediate revocation on leak)
- A concrete use case that leverages Vault PKI emerges (e.g., presenting short-lived client certs to out-of-mesh services)

The first-choice adoption architecture is **using istio-csr to make Vault the Istio root CA** (so that all certs, including workload certs, flow through Vault). The trade-off is that Istio startup depends on Vault; the decision is then made based on Vault HA maturity at that time.

## Consequences

### Positive

- No additional TLS-related operational surface (Istio automatic mTLS + cert-manager + Let's Encrypt remains)
- A Vault outage does not affect cert renewals (independent failure domain)
- The Phase 2 scope of Vault adoption stays light (concentrate on KV mount + ESO + DB engine alone)

### Trade-offs

- The "centrally managed certs" headline is unavailable. As CFP / dev.to material, the story shifts to "the design judgment of intentionally not adopting"
- If audit/compliance requirements come later, istio-csr adoption effort is incurred
- If there is a learning motivation to try Vault PKI, it cannot be experimented with inside kensan-lab; a separate test cluster would be required

## References

- ADR-001: TLS Termination Pattern (external -> Gateway TLS termination)
- ADR-008: Keycloak DB Credentials Are Not Moved to Vault (the same "do not move everything to Vault" principle)
- Design source: `kensan-workspace/projects/kensan-lab/secrets-phase1-design.md` § TLS ownership organization (rationale for not adopting Vault PKI) / § Future Vault PKI adoption scenario: via istio-csr
- [istio-csr](https://cert-manager.io/docs/usage/istio-csr/)
- [HashiCorp Vault PKI Secrets Engine](https://developer.hashicorp.com/vault/docs/secrets/pki)
