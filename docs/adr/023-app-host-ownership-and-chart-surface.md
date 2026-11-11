# ADR-023: App host ownership widens to `{name}-{label}`, and app-base exposes the Pod surface

## Status

**Accepted** (2026-11-11). Decided while designing the markport viewer deployment, the first
time an existing app (kensan) needed a second component and a second host.

## Context

Adding a component to an existing app should only touch that app's files. Deploying a
read-only viewer next to kensan instead required four platform-owned edits, because two
assumptions are baked into platform files:

- **One app = one release = one host.** The `app-httproute-ownership` ValidatingAdmissionPolicy
  allowed `app-{name}` to route `{name}.app.yu-min3.com` and nothing else, so a second host
  meant adding a per-app exception to a shared policy (the same shape `kensan.yu-mins.com`
  already has).
- **A field missing from the chart is treated as forbidden.** app-base had no way to declare an
  init container, a sidecar or Pod annotations/labels, so an app needing one had to change the
  platform chart first. The actual restrictions (no root, no privilege escalation, …) are already
  enforced by Pod Security restricted on every app namespace, and they apply to every container
  in the Pod; the chart's omissions added nothing to that.

Two further per-app touches (Keycloak redirect URIs in `bootstrap/keycloak/setup.sh`, and the
release list in the Application CR) come from the same root cause but are larger changes; they
are deferred below.

## Decision

1. **`app-{name}` owns `{name}.app.yu-min3.com` and `{name}-{label}.app.yu-min3.com`.** The
   second form adds exactly one DNS label (regex `{name}-[a-z0-9]([-a-z0-9]*[a-z0-9])?`), so a
   route cannot claim deeper names, other apps' names or other domains. `gateway-prod`'s
   certificate (`*.app.yu-min3.com`) already covers these hosts. External hosts
   (`*.yu-mins.com`) stay explicit platform exceptions.
2. **App names are a single word: `app-[a-z0-9]+`.** Without this, `app-kensan-markport` could
   claim the same host as `app-kensan`'s `kensan-markport`. A second ValidatingAdmissionPolicy
   (`app-namespace-name`, `failurePolicy: Fail`, `Deny`) rejects creating any other `app-*`
   namespace, and the Backstage template's name pattern matches it. This refines the
   `app-{name}` convention of [ADR-006](006-namespace-naming.md) /
   [ADR-014](014-namespace-naming-label-contract-v2.md): ADR-014 holds that names carry no
   contract weight, but for app namespaces the name already decided host ownership; this ADR
   makes that weight explicit and unambiguous. All existing app namespaces (canary, kensan,
   konro, koubou) comply.
3. **app-base exposes the Pod surface: `initContainers`, `extraContainers`, `podAnnotations`,
   `podLabels`.** Values pass through unchanged and default to empty, so existing releases render
   identically. `podLabels` never enter the selector (Deployment selectors are immutable).
4. **Restrictions live in policy, not in chart omissions.** What a Pod may do is decided by the
   namespace's Pod Security restricted level — today the PSA `enforce: restricted` label, with
   Kyverno `pss-restricted` running in Audit until its Phase 3 promotion ([ADR-012](012-policy-enforcement-kyverno.md)).
   A missing chart field is not a policy and should be added rather than worked around.
5. **app-base can render the app namespace (`namespace.create`, default false).** Adopted from
   the #541 draft. Existing apps keep their raw `namespace.yaml`; when several app-base releases
   share a namespace, only one may enable it.

## Alternatives considered

- **Per-app exceptions in the ownership policy** (`app-kensan` may also use
  `markport.app.yu-min3.com`): rejected — every second host becomes a platform edit, which is the
  problem being solved.
- **Subdomains per app** (`markport.kensan.app.yu-min3.com`): rejected — the wildcard
  certificate covers one level only, so every app would need its own certificate.
- **A double-hyphen separator** (`kensan--markport`) to keep hyphens in app names: rejected —
  awkward hostnames for every app to keep an option no existing app uses.
- **Enforcing the naming rule in Kyverno `ns-label-contract`:** rejected — host ownership is a
  security boundary (LAN DNS trusts route hostnames), and those Kyverno policies run with
  `failurePolicy: Ignore`; the admission policy fails closed.

## Deferred

- **Keycloak redirect URIs (`setup.sh`).** Each host still needs a manual entry and re-run.
  A reconciler that projects HTTPRoute hostnames into the gateway client would remove it, but it
  needs `manage-clients` on the realm and deserves its own decision. The alternatives already
  rejected in [ADR-021](021-gateway-prod-default-protected.md) (hostname wildcards, a single
  static callback) still do not fit: gateway-prod serves two cookie domains.
- **Release composition in the Application CR.** A second component still adds a source to the
  platform-owned Application. Moving composition into the app directory (an umbrella chart that
  uses app-base once per component) needs a trial on app-canary first.

## Consequences

- Adding markport to kensan touches the platform in two one-line places (`setup.sh` host,
  Application source) instead of four, and both are listed above for removal.
- New app names with hyphens are rejected at namespace creation. The kind explore template still
  accepts them because that environment has no LAN DNS and no ownership policy.
- `scripts/test_lan_dns.py` covers the widened rule: allowed `{name}-{label}`, denied deeper
  names, prefix look-alikes (`{name}x`), other apps' names, external domains and hyphenated app
  namespaces.
