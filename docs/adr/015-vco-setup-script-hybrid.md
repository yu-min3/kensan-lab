# ADR-015: Vault Config Operator + Setup-Script Hybrid

## Status

Accepted

> This is the ADR that `docs/secret-management/index.md` referenced as "ADR-012 (起票予定)" before
> ADR-012 was taken by the Kyverno policy-enforcement decision. The hybrid itself has been operating
> since the vault-transit chart work (PR #340); this document records the decision.

## Date

2026-06-07 (practice established with PR #340)

## Context

Vault-side configuration (mounts, roles, policies, engines) is GitOps-managed through the
**Vault Config Operator (VCO)** wherever possible: `SecretEngineMount`, `DatabaseSecretEngineConfig/Role`,
`KubernetesAuthEngineRole`, `Policy` CRs live under `kubernetes/secrets/vault-*-engine/` and are synced by
Argo CD like everything else.

However, VCO does not cover everything we need:

| Vault operation | VCO gap | Current workaround |
|---|---|---|
| Transit key create / rotate | No `TransitSecretEngine`-family CR exists | `kubernetes/secrets/vault-transit-engine/temp/setup-transit-keys.sh`, run once per new key; rotate manually via `vault write -f transit/keys/<name>/rotate` |
| OIDC config (`auth/oidc/config`) | VCO patch unsupported; full-write of provider URL etc. required | `temp/post-bootstrap-oidc-ux.sh` (Stage 4) full-write |

### Options considered

#### Pattern A: Scripts everywhere (drop VCO)

Manage all Vault config via bootstrap shell scripts / Terraform.

- Pros: one mechanism, no operator to run
- Cons: loses GitOps reconciliation (drift detection, self-heal) for the 90% VCO covers well;
  scripts are imperative and unaudited by Argo CD

#### Pattern B: Replace VCO with a fuller tool (Terraform provider, bank-vaults, ...)

- Pros: potentially closes the gaps
- Cons: migration cost for all working VCO CRs; each alternative has its own gap profile and operational
  surface; nothing offered Transit CRs *and* the lightweight in-cluster reconcile model we already have

#### Pattern C: Hybrid — VCO for what it supports, one-shot scripts for the gaps (Adopted)

- Pros: keeps GitOps reconciliation where it works; gap surface is small (2 operations), explicit, and
  documented in one table; scripts are one-shot bootstrap, not steady-state operations
- Cons: two mechanisms; the script-managed slice is invisible to Argo CD drift detection

## Decision

**Adopt Pattern C.** VCO CRs are the default for all Vault configuration; setup scripts are the documented
exception, constrained as follows:

1. **The exception table in [`docs/secret-management/index.md`](../secret-management/index.md) is the
   exhaustive list** of script-managed Vault state. Adding a script requires adding a row there.
2. Scripts must be **one-shot / idempotent bootstrap** operations, committed to the repo next to the
   capability they bootstrap. No steady-state reconciliation via scripts.
3. **Removal trigger**: when VCO (or a successor) ships CR support for a gap (e.g. a
   `TransitSecretEngine` / transit-key CR, or OIDC config patch semantics), migrate that row from script
   to CR and delete the script in the same PR. Re-evaluate this ADR if the exception table grows beyond
   ~4 rows — at that point Pattern B's migration cost may be worth paying.

## Consequences

### Positive

- GitOps reconciliation retained for all engine mounts / roles / policies (the high-churn surface)
- The non-GitOps slice is enumerable in one table instead of being implicit tribal knowledge
- A clear, pre-agreed trigger line for retiring each script

### Trade-offs

- Transit key rotation stays manual until VCO grows a CR for it
- Anyone auditing "what configures Vault" must read both the CRs and the exception table

## References

- `docs/secret-management/index.md` §例外 — the exception table (SoT for script-managed state)
- `kubernetes/secrets/vault-transit-engine/README.md` — the capability that motivated the hybrid (PR #340/#343)
- [ADR-012](012-policy-enforcement-kyverno.md) — unrelated; took the number this decision was originally
  penciled in for
