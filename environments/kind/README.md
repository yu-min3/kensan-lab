# environments/kind — the explore layer

A subset of this platform that stands up on a kind cluster in about ten minutes,
so that someone can look at how it is composed without owning the hardware.
User-facing instructions are in
[docs/getting-started/try-it-with-kind.md](../../docs/getting-started/try-it-with-kind.md);
this file is for whoever has to keep it working.

## What is here

```
environments/kind/
  kind-cluster.yaml      single node, host ports 80/443, hardware-class label
  versions.sh            the one version that cannot live in an Application
  explore-root-app.yaml  app-of-apps entrypoint; takes repo + revision
  applications/          in-repo Helm chart rendering the explore Applications
  values/                values layered on top of the bare-metal ones
  resources/             the substitution layer — the only genuinely new manifests
```

Argo CD is installed by `scripts/explore-up.sh` with Helm, because nothing can
sync it before it exists. Everything after that arrives through
`explore-root-app.yaml` and behaves like the real cluster.

The script does three other things Argo CD cannot, all for the same reason —
they have to happen either before there is a cluster to sync into, or after
something Argo CD created is already running:

| | why it is not an Application |
|---|---|
| **generates every SSO credential** | a client secret committed to a public repository is a working credential against every cluster started from it |
| **creates the Keycloak realm with `kcadm`** | Keycloak does not substitute environment variables during realm import — verified: `${VAR}` in a client secret is stored as those six characters — so a realm file in git would have to carry real secrets |
| **rewrites CoreDNS** | the issuer hostname has to resolve to the gateway from inside a pod and to `127.0.0.1` from the browser; that is one name with two answers |

Bare metal solves the first two the same way, with `bootstrap/keycloak/setup.sh`
run once by hand. Explore runs its own smaller version automatically because
there is nobody to run it and nothing worth keeping if it goes wrong.

## Why an Application per component instead of a patch

The original design was to reference the bare-metal `app.yaml` files from a
kustomization and append a values file with a JSON6902 patch, so no spec would
be duplicated. That does not work, and the reasons are worth recording so nobody
tries it again:

- Kustomize's default load restrictor refuses plain **file** references outside
  the kustomization root, which is what `../../../kubernetes/argocd/...` is.
  Directory references are allowed, but only when the directory holds a
  `kustomization.yaml`.
- Putting `kustomization.yaml` files into `kubernetes/argocd/applications/`
  breaks production: `platform-root` syncs that tree as a **directory** source
  with `recurse: true`, so it would try to apply `kind: Kustomization` to the
  API server.
- Relaxing the restrictor repo-wide (`--load-restrictor LoadRestrictionsNone` in
  `argocd-cm`) would weaken a security boundary on the real cluster for the
  benefit of a demo.

So the specs are duplicated, deliberately, and the risk that comes with
duplication — a chart version silently drifting apart — is converted into a CI
failure instead.

## Anti-drift

Every explore Application names the bare-metal Application it mirrors:

```yaml
metadata:
  annotations:
    kensan-lab.platform/mirrors: kubernetes/argocd/applications/network/istiod/app.yaml
```

`scripts/validate_argocd_apps.py` reads that annotation and fails the build when
the upstream chart name, chart repository or version disagree.

The annotation may also point at an **ApplicationSet**, which is how
`prometheus` and `grafana` are checked: bare metal generates all six
observability Applications from one ApplicationSet, whose template says
`chart: '{{ .chart }}'` and so cannot be compared directly. The validator
follows the generator to the `config.json` the Application is named after and
compares against that instead.

`versions.sh` is a knowing exception to the repository rule that chart versions
live in an Application's `targetRevision`: Argo CD's own chart version is needed
before Argo CD exists, so `explore-up.sh` has to read it from somewhere else.
The same validator checks that file against the bare-metal Argo CD Application,
so the exception cannot drift either. Nothing else belongs in `versions.sh`.

What is **not** checked, on purpose: which components explore runs, and what
values it layers on. Explore is a subset with substitutions; those are supposed
to differ.

## Maintenance

1. **Adding a component to the real cluster does not add it here.** If you want
   it in explore, add one template under `applications/templates/`. Explore not
   growing on its own is the feature — it is what keeps this from becoming a
   second platform to maintain.
2. **Chart versions come from the bare-metal Application.** Never bump one only
   on the explore side; the drift guard will stop you, and the guard is right.
3. **When Explore CI goes red, ask whether the production change is still
   reproducible before you touch anything in here.** This job exists to catch
   changes that quietly acquire a dependency on hardware, on Cilium, or on
   Vault. Making explore green again without answering that question throws away
   the signal.
4. **The substitutions are supposed to differ from their bare-metal
   counterparts.** `gateway-explore.yaml`, `storageclass-longhorn.yaml` and
   `tls-selfsigned.yaml` are not out-of-date copies. Do not sync them up.
   `tls-selfsigned.yaml` in particular has no bare-metal counterpart to sync to:
   production's issuer is a Let's Encrypt `ClusterIssuer` doing a DNS-01
   challenge, and the two are alternatives rather than versions of each other.
5. **Nothing explore-related goes in `kubernetes/argocd/applications/`.**
   `platform-root` syncs that directory with `recurse: true`; a file left there
   lands on the real cluster.
6. **Do not hardcode a new site-specific value into a raw manifest.** Domains,
   IPs and issuer URLs are the reason three of the files here exist at all, and
   nothing is coming to remove them. Every new one is permanent, so this is the
   single rule that decides whether this layer ages well.
7. **If CI time becomes a problem, shrink the slice rather than moving the job
   to nightly.** A slow signal on every pull request is worth more than a fast
   one nobody reads, and "while we are here, let us also run X in explore" is
   how the slice gets slow.
8. **Promoting Kyverno to Enforce (ADR-012 Phase 3) means revisiting
   `values/istiod.yaml`.** The trimmed requests there are sized for Audit mode;
   under Enforce a workload that asks for too little can be rejected outright.
9. **Do not edit `scripts/explore-up.sh` while it is running.** bash reads a
   script incrementally, so an edit mid-run shifts the offset it is reading from
   and produces a syntax error on a line that is perfectly valid. It looks like
   a bug in the script and is not.
10. **A dashboard whose metrics do not exist here does not belong here.** Explore
   ships one of production's three because the other two read etcd, Cilium and
   an OTel collector that a single kind node has none of. Half-empty panels do
   not demonstrate observability; they demonstrate a broken cluster, which is
   the opposite of the point. The same test applies to scrape jobs — a target
   that can only ever be down is worse than an absent one.

## Why the substitutions are permanent

Eight of the files here exist because a hostname is hardcoded in a raw
manifest — the Gateway, the routes for Argo CD, Backstage, Grafana and Keycloak,
the wildcard certificate, Keycloak's env config, and the authorization policy
naming the protected host. For a while the plan was to centralise those values behind
a `site.yaml` and delete these copies. **That is no longer planned**, so they
stay.

The reasoning, in short: centralising was justified by making the repository
easier to fork, and forking is not what this repository is for. The place
somebody tries the platform is here, on kind, where no domain is needed at all
(`*.127-0-0-1.sslip.io` resolves for everyone). The bare-metal side stays the
author's own configuration. Rewriting the Gateway, the routes, the
AuthorizationPolicies and the certificates to be site-driven would have meant a
broad refactor of the production network and auth layers — where a missed host
means an unauthenticated route and a rejected one means an unreachable service
— in exchange for deleting three files from a throwaway cluster.

Repositories that do solve this, like
[onedr0p/cluster-template](https://github.com/onedr0p/cluster-template), are
templates you generate a repository *from*: a single `cluster.toml` and a
render step at bootstrap, with plain manifests on the other side. That is a
different kind of repository than a reference architecture, and copying its
answer here would mean becoming one.

So: expect these eight to stay, keep them small, and keep new site-specific
values out of raw manifests anyway — rule 6 above still holds, because it is
what keeps the count from growing past the hostnames that genuinely need one.

## The one component that is genuinely copied

`resources/backstage/deployment.yaml` is the largest single divergence in this
directory, and the only Deployment explore owns rather than follows. The
bare-metal one reads a PostgreSQL password and a GitHub token from Secrets that
External Secrets fills from Vault; without Vault those Secrets never exist and
the pod never starts, so the references had to go.

Everything not forced to differ is copied verbatim — image, port, probes,
resource requests — and the header of that file lists the three deliberate
differences. When it drifts from the bare-metal Deployment, that is a bug, not a
feature: check the two side by side before changing either.

Backstage is also the first workload here that runs an Istio sidecar, because
the shared namespace carries `istio-injection: enabled`. Nothing else in the
slice joins the mesh data plane.
