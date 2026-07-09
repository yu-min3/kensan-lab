# Identity & RBAC model

Who exists in the identity system, which groups they belong to, and what role that membership becomes inside each application. The [Gateway OIDC guide](./gateway-oidc.md) answers "which groups may *reach* which host"; this page answers "**what are you once you're in**".

## The identity source

| Item | Value |
|---|---|
| IdP | Keycloak, realm `kensan` (self-hosted, local user store — **no upstream federation**; identities live here, not in Google/GitHub) |
| Human users | `yu` (currently the only one), member of `platform-admin` |
| Groups | `platform-admin` (full operator) / `platform-dev` (reduced role — **defined but currently empty**, reserved for future app developers) |
| Claim propagation | each OIDC client carries an `oidc-group-membership-mapper` that puts the `groups` claim on the id_token |
| Provisioning | realm / groups / users / clients created by `bootstrap/keycloak/setup.sh` |

So the honest current state: **one human, one group, and effectively admin everywhere** — but every mapping below is already two-tier, so adding a `platform-dev` user requires zero per-app work.

## How a group becomes a role (the federation map)

```mermaid
%%{init: {"flowchart": {"htmlLabels": true}} }%%
flowchart LR
    U["user: yu"] -->|member of| G["Keycloak group<br/>platform-admin"]
    G -->|"groups claim<br/>on id_token"| CLAIM(( ))

    CLAIM --> GW["Istio Gateway ALLOW<br/>reach Cat-2 + Cat-3 hosts"]
    CLAIM --> ARGO["ArgoCD<br/>role:admin (policy.csv)"]
    CLAIM --> GRAF["Grafana<br/>Admin (role_attribute_path)"]
    CLAIM --> VLT["Vault<br/>identity group → policy 'admin'"]

    classDef default fill:#26221D,stroke:#4A4232,color:#FCFAF6
    classDef idp fill:#075985,stroke:#38BDF8,color:#FCFAF6
    classDef app fill:#2F6B45,stroke:#7AC99A,color:#FCFAF6
    class U,G idp
    class GW,ARGO,GRAF,VLT app
```

One group membership, evaluated independently by each consumer. There is no shared session between the apps — what's shared is the **claim**, and each app maps it with its own mechanism:

| Consumer | Mapping mechanism | `platform-admin` | `platform-dev` | any other authenticated user |
|---|---|---|---|---|
| **Gateway** Cat-2 hosts (Backstage, Prometheus) | Istio `AuthorizationPolicy` on the groups claim | ✅ reach | ✅ reach | ❌ denied at the gateway |
| **Gateway** Cat-3 hosts (Hubble, Longhorn) | same | ✅ reach | ❌ | ❌ |
| **ArgoCD** | `rbac.policy.csv` (`scopes: [groups]`) | `role:admin` | `role:readonly` | no role mapped → no access (no default role) |
| **Grafana** | `role_attribute_path` (JMESPath on groups) | `Admin` | `Editor` | `Viewer` |
| **Vault** | OIDC role `default` `bound_claims` + external identity group → policy | policy `admin` (token TTL 1h / max 8h) | ❌ **login rejected** (`bound_claims` allows only platform-admin) | ❌ login rejected |
| **Backstage** | — (guest provider) | see note below | | |

Notes on the two ends of the spectrum:

- **Vault is the strictest**: membership is checked at *login* (`bound_claims`), not just at role-mapping — a non-admin can't even get a token. The role attaches no policies directly; policy `admin` flows through the external identity group `platform-admin` (`kubernetes/auth/vault-oidc-auth/`, VCO-managed), so group membership in Keycloak is the single switch.
- **Backstage is the loosest (known gap)**: the gateway guarantees only `platform-admin` / `platform-dev` can reach it, but the app itself runs the **guest auth provider** — the SSO identity is not yet consumed for in-app authorization. Wiring the `X-Auth-Request-*` headers into a Backstage proxy auth provider is future work.

## Break-glass accounts (outside SSO)

Every SSO path above dies with Keycloak — by design there are local escape hatches, stored in the password manager:

| System | Account | Mechanism | Why it exists |
|---|---|---|---|
| Vault | `emergency-admin` | userpass auth, policy `admin` | recovering Vault/Keycloak coupling failures — [proved its worth in the 2026-06-06 incident](../incidents/2026-06-06-vault-oidc-credential-drift.md) |
| ArgoCD | `admin` | built-in local account (chart default) | GitOps repair when SSO is down |
| Grafana | `grafana-admin` | local admin (ESO-delivered secret) | dashboard access without SSO |
| Keycloak | `KEYCLOAK_ADMIN` (master realm) | bootstrap admin | administering the IdP itself |

## Design intent

- **Two tiers, not N**: `platform-admin` (operate everything) and `platform-dev` (read/edit application-level surfaces). Finer-grained roles are deliberately deferred until a second human actually exists — empty RBAC taxonomy is maintenance without benefit.
- **The mapping lives with each app's config, the membership lives in Keycloak**: adding a user to `platform-dev` instantly yields Gateway Cat-2 reach + ArgoCD readonly + Grafana Editor, with Vault access still denied — a usable "developer" profile out of the box.
- The authorization model's history: [ADR-002](../adr/002-authentication-authorization-architecture.md) (hybrid gateway + per-service) / [ADR-010](../adr/010-istio-native-oauth2-absent.md) (oauth2-proxy as the gateway mechanism) / [ADR-016](../adr/016-lan-frictionless-cf-access-external-gate.md) (session length & external gate).

## Related

- [Gateway OIDC guide](./gateway-oidc.md) — host × group reachability matrix, add-a-host checklists
- [oauth2-proxy](./oauth2-proxy.md) / [ArgoCD ↔ Keycloak](./argocd-keycloak-integration.md)
- Architecture summary: [`kubernetes/auth/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/kubernetes/auth/README.md)
