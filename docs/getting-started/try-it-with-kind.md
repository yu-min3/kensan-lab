# Try it with kind

This repository runs a Kubernetes platform on bare metal — three Raspberry Pi 5
workers and an x86 mini PC in a cupboard. You cannot borrow the cupboard, so
there is a second way in: one command stands up the same GitOps tree on a kind
cluster on your laptop, using the same Argo CD Applications, the same Helm
values, and the same policy set as the real thing.

```console
$ git clone https://github.com/yu-min3/kensan-lab && cd kensan-lab
$ make try
```

Roughly ten minutes later you have Argo CD, Istio with Gateway API, Kyverno
running the production policies, and a demo app reachable in a browser.

```console
$ make explore-down    # when you are done
```

## What you get

| | |
|---|---|
| **Argo CD** | `http://argocd.127-0-0-1.sslip.io` — the app-of-apps tree, one Application per component, exactly as on bare metal |
| **Istio + Gateway API** | The demo app is reached through a real `Gateway` and `HTTPRoute`, not a port-forward |
| **Kyverno** | The production `ClusterPolicy` set, in Audit mode — `kubectl get policyreport -A` |
| **A demo app** | Deployed by `charts/app-base`, the same chart the real apps use, with a values file and no chart changes |

## Prerequisites

`docker`, `kind`, `kubectl`, `helm`, `curl`, and `make`.

```console
$ brew install kind kubectl helm            # macOS
```

Give Docker at least **6 GiB of memory**. Docker Desktop defaults to 4 GiB,
which is not enough for Istio's control plane and Kyverno's webhook on one node
— `explore-up.sh` checks this before it starts, so you find out in a second
rather than in a Pending pod twenty minutes later.

Host ports **80** and **443** must be free; the gateway is published on them so
that the URLs above work without a proxy.

`*.127-0-0-1.sslip.io` is public DNS that resolves to `127.0.0.1`, which is what
keeps this from asking you to edit `/etc/hosts`. If you are offline, add the
names yourself:

```console
$ echo "127.0.0.1 argocd.127-0-0-1.sslip.io demo.127-0-0-1.sslip.io" | sudo tee -a /etc/hosts
```

## How it is put together

The explore layer is a **subset with substitutions**, not a fork. Every
component it runs is the same Application file the bare-metal cluster runs,
pointed at the same chart version and the same values.

Two things differ, and both are contained in `environments/kind/`:

- **What is left out.** No Cilium, Vault, Keycloak, Longhorn, cert-manager,
  Cloudflare Tunnel, or observability stack. Those either need hardware, need
  credentials you do not have, or would push the ten minutes out to thirty.
- **What is swapped.** The Cilium L2 load balancer becomes a kind port mapping;
  Longhorn becomes kind's local-path provisioner wearing the name `longhorn`, so
  that the repository's PVCs bind unmodified; the wildcard TLS certificate
  becomes plain HTTP.

The substitutions live in exactly two files, `resources/gateway-explore.yaml`
and `resources/storageclass-longhorn.yaml`, and both are meant to be deleted
later — see [the maintenance
notes](https://github.com/yu-min3/kensan-lab/blob/main/environments/kind/README.md).

Every pull request that touches `kubernetes/`, `charts/` or `environments/`
stands this cluster up in CI and fails if the platform stops coming up. That is
the real reason the layer exists: it makes "still reproducible from scratch" a
property CI checks rather than a claim in a README.

## What kind cannot show

These are not gaps waiting to be filled. Each one is impossible in a way worth
being precise about, and each is a reason the bare-metal cluster is where the
interesting parts live.

**The load balancer.** Cilium announces a virtual IP by answering ARP on the
LAN. kind's nodes are containers on a Docker bridge; there is no physical
segment to announce into and no other machine that could ask. The substitute is
a port mapping, which gets traffic to the gateway but cannot show you a VIP
failing over between nodes.

**Single sign-on.** Keycloak, oauth2-proxy and the `ext_authz` chain need an
issuer with a resolvable public hostname and a set of client secrets. Phase 3
of [the plan](https://github.com/yu-min3/kensan-lab) brings a dev-mode Vault and
a realm import to kind; today the Argo CD you get here uses its local admin
account.

**The root of trust for secrets.** This one cannot be solved by more work. The
sealed secrets in this repository are encrypted to *our* controller's key and
are useless to anyone else. Vault's unseal keys and root token come into
existence during `vault operator init` and are never written down anywhere they
could be committed. Generating them from Terraform would only move the secret
into Terraform state. The irreducible part of bootstrapping a platform is that
somebody has to run a script and keep what it prints.

**Trusted certificates.** The real gateway terminates TLS with a wildcard
certificate issued through a DNS-01 challenge against a domain we own. A fork
has neither. Phase 2 will add a self-signed issuer and let the browser warn you,
because the alternative — `mkcert` rewriting your system trust store — is more
than a one-command demo should do to your laptop.

**Anything about failure.** One node cannot rebuild a Longhorn replica, cannot
be drained, and cannot lose an L2 lease. The explore cluster shows you how the
platform is *composed*. How it *behaves* is the bare-metal cluster's story, and
it is told in [the incident
notes](https://github.com/yu-min3/kensan-lab/tree/main/docs/incidents).

**Multiple architectures.** kind's nodes are all your host's architecture, so
the arm64/amd64 split that the real cluster schedules around cannot be
reproduced. The two apps that are amd64-only are left out for that reason; on
Apple Silicon they would emulate, slowly.

## Troubleshooting

**A pod is Pending with insufficient memory.** Docker has less RAM than it
needs. Raise it to 6 GiB or more and run `make explore-down && make try`.

**Ports 80 or 443 are in use.** Find the process with
`sudo lsof -nP -iTCP:80 -sTCP:LISTEN`. A previous explore cluster is the usual
culprit — `make explore-down` clears it.

**Application pods start but have no Istio sidecar.** istio-cni chains onto
kindnet, and `cni.exclusive` is pinned off in
`environments/kind/values/istio-cni.yaml` for exactly this reason. Check the
CNI config the node ended up with:

```console
$ docker exec kensan-lab-explore-control-plane ls /etc/cni/net.d
```

**An Application will not go Healthy.** `explore-up.sh` prints which one it is
waiting for. Then:

```console
$ kubectl -n argocd describe application <name>
```

**The demo app returns 404.** The route was rejected by the listener. Gateways
in this repository admit routes by namespace label, so a namespace without
`kensan-lab.platform/environment` gets `NotAllowedByListeners`:

```console
$ kubectl -n app-demo get httproute demo -o jsonpath='{.status.parents}' | jq
```

**Everything is healthy but nothing loads in the browser.** DNS. Check that
`demo.127-0-0-1.sslip.io` resolves; if not, use the `/etc/hosts` fallback above.

## Where to go next

- [Infrastructure overview](../architecture/infrastructure.md) — what the real
  cluster is made of
- [Argo CD architecture](../architecture/argocd.md) — the app-of-apps structure
  you just watched sync
- [ADR-012: policy enforcement with Kyverno](../adr/012-policy-enforcement-kyverno.md)
  — why those policies are in Audit and what promoting them means
