# Architecture Decision Records

ADRs (Architecture Decision Records) capture significant design choices made for the platform — what was decided, what alternatives were considered, and why. They serve as the **source of truth for *why* the platform looks the way it does**, even after the code itself has moved on.

## Format

Each ADR follows a lightweight structure:

- **Status** — Proposed / Accepted / Superseded / Deprecated
- **Context** — What forced a decision
- **Decision** — What was chosen
- **Consequences** — What this enables, what it costs, and what to watch out for

## Index

| # | Title | Status |
|---|---|---|
| 001 | [TLS Termination Pattern](001-tls-termination-pattern.md) | Accepted |
| 002 | [Authentication & Authorization Architecture](002-authentication-authorization-architecture.md) | Accepted |
| 003 | [ApplicationSet Migration Strategy](003-applicationset-migration-strategy.md) | Accepted (see Addendum) |
| 004 | [Network Policy Design](004-network-policy-design.md) | Partially superseded by 009 |
| 005 | [Istio Native OAuth2](005-istio-native-oauth2.md) | Superseded by 010 |
| 006 | [Namespace Naming](006-namespace-naming.md) | Partially superseded by 014 |
| 007 | [No Vault PKI](007-no-vault-pki.md) | Accepted (see Addendum) |
| 008 | [Keycloak DB Credentials](008-keycloak-db-credentials.md) | Superseded by 013 |
| 009 | [Shared `allow-istio` NetworkPolicy](009-shared-allow-istio-network-policy.md) | Accepted |
| 010 | [Istio Native OAuth2 (Absent)](010-istio-native-oauth2-absent.md) | Accepted |
| 011 | [Vault Image Tag Explicit Pin](011-vault-version-pinning.md) | Accepted |
| 012 | [Policy Enforcement with Kyverno](012-policy-enforcement-kyverno.md) | Accepted |
| 013 | [Keycloak DB Credentials → Vault Dynamic](013-keycloak-db-credentials-vault-dynamic.md) | Accepted |
| 014 | [Namespace Naming & Label Contract v2](014-namespace-naming-label-contract-v2.md) | Accepted |
| 015 | [VCO + Setup-Script Hybrid](015-vco-setup-script-hybrid.md) | Accepted |
| 016 | [LAN-Frictionless Sessions + External-Gate Model](016-lan-frictionless-cf-access-external-gate.md) | Accepted in part (gate deferred) |
| 017 | [Remove kensan-legacy (tag archive)](017-kensan-legacy-removal.md) | Accepted |
| 018 | [Backstage manifests placement](018-backstage-manifests-placement.md) | Proposed |
