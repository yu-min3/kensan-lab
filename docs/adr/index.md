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
| 003 | [ApplicationSet Migration Strategy](003-applicationset-migration-strategy.md) | Accepted |
| 004 | [Network Policy Design](004-network-policy-design.md) | Accepted |
| 005 | [Istio Native OAuth2](005-istio-native-oauth2.md) | Superseded by 010 |
| 006 | [Namespace Naming](006-namespace-naming.md) | Accepted |
| 007 | [No Vault PKI](007-no-vault-pki.md) | Accepted |
| 008 | [Keycloak DB Credentials](008-keycloak-db-credentials.md) | Accepted |
| 009 | [Shared `allow-istio` NetworkPolicy](009-shared-allow-istio-network-policy.md) | Accepted |
| 010 | [Istio Native OAuth2 (Absent)](010-istio-native-oauth2-absent.md) | Accepted |
| 011 | [Vault Image Tag Explicit Pin](011-vault-version-pinning.md) | Accepted |
