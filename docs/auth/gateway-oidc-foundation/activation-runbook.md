# Gateway OIDC Activation Runbook

> Step-by-step procedure to take this scaffolding live, per Path. Read
> ADR-010 first.

## Common pre-checks (all Paths)

```bash
# Confirm Keycloak realm + groups
kubectl -n platform-auth-prod exec -it deployment/keycloak -- bash -c '
  /opt/keycloak/bin/kcadm.sh config credentials \
    --server http://localhost:8080 --realm master \
    --user admin --password "$KEYCLOAK_ADMIN_PASSWORD" >/dev/null
  /opt/keycloak/bin/kcadm.sh get groups -r kensan --fields name
  /opt/keycloak/bin/kcadm.sh get realms/kensan --fields realm,sslRequired,frontendUrl
'

# Confirm gateway-platform is healthy
kubectl get gateway gateway-platform -n istio-system -o yaml \
  | grep -A2 "type: Programmed"

# Confirm OIDC discovery from outside cluster
curl -s https://auth.platform.yu-min3.com/realms/kensan/.well-known/openid-configuration \
  | jq -r '.issuer, .authorization_endpoint, .token_endpoint'
```

Expected:
- Groups `platform-admin`, `platform-dev`
- Realm `kensan`, `sslRequired: external`
- issuer `https://auth.platform.yu-min3.com/realms/kensan`

## Path A: oauth2-proxy via envoy_ext_authz_http (recommended)

### Step A1 — Keycloak client

Uncomment the Path A block in `bootstrap/keycloak/setup.sh` and run it:

```bash
cd /Users/yu/kensan-workspace/projects/kensan-lab/kensan-lab
export BW_SESSION=$(bw unlock --raw)
./bootstrap/keycloak/setup.sh
# vault client / user は skip され、istio-gateway-platform だけが新規作成される
```

The script saves `client_secret` to Bitwarden as
`kensan-lab/keycloak/oidc-client-istio-gateway-platform`.

### Step A2 — Vault key material

```bash
# Pull client_secret from Bitwarden
CLIENT_SECRET=$(bw get item kensan-lab/keycloak/oidc-client-istio-gateway-platform | jq -r .login.password)
COOKIE_SECRET=$(openssl rand -base64 32 | tr -d '\n')

# Put into Vault (path: secret/platform-auth/istio-gateway/platform)
vault kv put secret/platform-auth/istio-gateway/platform \
  client_id=istio-gateway-platform \
  client_secret="$CLIENT_SECRET" \
  cookie_secret="$COOKIE_SECRET"

# Verify
vault kv get -format=json secret/platform-auth/istio-gateway/platform \
  | jq '.data.data | keys'
```

### Step A3 — oauth2-proxy Helm app (separate PR — out of scope here)

This Helm multi-source Application is NOT part of this PR. A follow-up
PR adds `infrastructure/security/oauth2-proxy/` with:
- `values.yaml` (using `path-a-oauth2-proxy-values.yaml` as the starting
  point)
- `resources/` (HTTPRoute for `oauth2-proxy.platform.yu-min3.com`,
  ExternalSecret pulling from Vault)
- ArgoCD `Application` CR pointing at `auth-system` namespace

### Step A4 — Add ext_authz extensionProvider

Edit `infrastructure/network/istio/istiod/values.yaml` and append the
content of `path-a-meshconfig-snippet.yaml` under `meshConfig:`. Example
final state:

```yaml
meshConfig:
  extensionProviders:
    - name: oauth2-proxy
      envoyExtAuthzHttp:
        service: oauth2-proxy.auth-system.svc.cluster.local
        port: 4180
        # ... see path-a-meshconfig-snippet.yaml
```

Commit + sync the istiod Application. Verify the merged config:

```bash
kubectl -n istio-system get cm istio -o yaml \
  | yq '.data.mesh' \
  | yq '.extensionProviders'
```

### Step A5 — Apply RequestAuthentication

Move (`git mv`) `docs/auth/gateway-oidc-foundation/requestauthentication.yaml`
to `infrastructure/network/istio/keycloak-jwt-platform.yaml` and commit.
ArgoCD picks it up via the istio-resources Application.

Verify:

```bash
kubectl get requestauthentication -n istio-system keycloak-jwt-platform -o yaml
```

### Step A6 — Apply ALLOW policy FIRST

Move `docs/auth/gateway-oidc-foundation/authorizationpolicy-allow.yaml`
to `infrastructure/network/istio/gateway-platform-allow.yaml`.
**Verify it contains every currently-routed host before committing**
(`kubectl get httproute -A` and cross-check).

Commit, sync, verify:

```bash
kubectl get authorizationpolicy -n istio-system gateway-platform-allow -o yaml
```

Test from a browser **before** going to step A7:
- Anonymous request to `argocd.platform.yu-min3.com` should now redirect
  to oauth2-proxy → Keycloak.
- After login, group claim must appear in oauth2-proxy logs.

### Step A7 — Apply CUSTOM action policy (binds ext_authz to Gateway)

Move `path-a-authorizationpolicy-custom.yaml` similarly. This is the
policy that actually invokes oauth2-proxy via ext_authz on traffic to
gateway-platform.

### Step A8 — (optional) Apply default-deny

Move `authorizationpolicy-deny-all.yaml`. Belt-and-suspenders only;
gates close further by being explicit.

### Rollback (Path A)

In reverse order:

```bash
kubectl delete authorizationpolicy -n istio-system gateway-platform-deny-all
kubectl delete authorizationpolicy -n istio-system gateway-platform-custom
kubectl delete authorizationpolicy -n istio-system gateway-platform-allow
kubectl delete requestauthentication -n istio-system keycloak-jwt-platform
# Then comment out the meshConfig.extensionProviders entry and let
# ArgoCD sync it back.
# Disable the oauth2-proxy Application via ArgoCD UI if needed.
```

## Path B: EnvoyFilter wrapping envoy.filters.http.oauth2

### Step B1 — Keycloak client

Uncomment the Path B block in `bootstrap/keycloak/setup.sh`. The
redirect_uris list every host on gateway-platform individually.

### Step B2 — Vault key material

Same as Path A Step A2 but with these additional fields:

```bash
HMAC_SECRET=$(openssl rand -base64 32 | tr -d '\n')
TOKEN_SECRET=$(openssl rand -base64 32 | tr -d '\n')

vault kv put secret/platform-auth/istio-gateway/platform \
  client_id=istio-gateway-platform \
  client_secret="$CLIENT_SECRET" \
  hmac_secret="$HMAC_SECRET" \
  token_secret="$TOKEN_SECRET"
```

### Step B3 — Apply EnvoyFilter

Move `path-b-envoyfilter.yaml` to `infrastructure/network/istio/`.

⚠️ Test extensively in a non-production gateway first. EnvoyFilter is
brittle across Istio upgrades.

### Step B4-B6 — Apply RequestAuthentication + ALLOW + deny-all

Same as Path A Step A5-A8. The Envoy oauth2 filter handles the redirect
itself; you do NOT need a CUSTOM-action policy.

### Rollback (Path B)

```bash
kubectl delete envoyfilter -n istio-system gateway-platform-oauth2
# Then RequestAuth/AuthZ as Path A.
```

## Path C: Per-service OIDC

This Path means abandoning Gateway-level OIDC. The foundation YAML in
this directory is not used. Instead each service receives its own
Keycloak OIDC client and does redirect/cookie itself:

| Service | OIDC support |
|---|---|
| Argo CD | native (`oidc.config` in argocd-cm) |
| Grafana | native (`auth.generic_oauth`) |
| Backstage | native (auth provider plugin) |
| Vault | already done (Stage 1) |
| Hubble | no native — basic auth or LAN-only NetworkPolicy |
| Prometheus | no native — basic auth or LAN-only NetworkPolicy |
| Longhorn | no native — basic auth or LAN-only NetworkPolicy |

The follow-up work for Path C is a per-service PR series, not a single
PR.
