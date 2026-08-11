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
   counterparts.** `gateway-explore.yaml` and `storageclass-longhorn.yaml` are
   not out-of-date copies. Do not sync them up.
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

## Why the substitutions are permanent

Three of the files here exist because a hostname is hardcoded in a raw
manifest — the Gateway, Argo CD's route and Backstage's route. For a while the
plan was to centralise those values behind a `site.yaml` and delete these
copies. **That is no longer planned**, so they stay.

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

So: expect these three to stay, keep them small, and keep new site-specific
values out of raw manifests anyway — the rule below still holds, because it is
what keeps the *count* at three.

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
