# Try it with kind

This repository runs a Kubernetes platform on bare metal — three Raspberry Pi 5
workers and an x86 mini PC in a cupboard. You cannot borrow the cupboard, so
there is a second way in: one command stands up a slice of the same GitOps tree
on a kind cluster on your laptop, using the same Argo CD Applications, the same
Helm values, and the same Kyverno policies as the real thing.

```console
$ git clone https://github.com/yu-min3/kensan-lab && cd kensan-lab
$ make try
```

Nothing else to configure. A few minutes later you have Argo CD reconciling
eleven Applications, Backstage serving the developer portal, Istio routing to
both through the Gateway API, and the production policy set reporting on
everything in the cluster.

```console
$ make explore-down     # when you are done
```

## What you get

| | |
|---|---|
| **Argo CD** | `http://argocd.127-0-0-1.sslip.io` — the app-of-apps tree, one Application per component, the same shape as bare metal |
| **Backstage** | `http://backstage.127-0-0-1.sslip.io` — the developer portal, with the golden path template and the service catalog |
| **Istio + Gateway API** | Both are reached through a real `Gateway` and `HTTPRoute`, not a port-forward |
| **Kyverno** | All six production `ClusterPolicy` objects, in Audit — verdicts land in `PolicyReport` within a minute |
| **A demo app** | Deployed by `charts/app-base`, the same chart the real apps use: a values file and no chart changes |
| **A `longhorn` StorageClass** | Not Longhorn, but named after it, so every PVC in this repository binds unmodified |

## Prerequisites

`docker`, `kind`, `kubectl`, `helm`, `curl` and `make`. `explore-up.sh` checks
for all of them before it starts, so a missing one costs you a second rather
than a confusing failure ten minutes in.

**macOS** — Docker Desktop from
[docker.com](https://www.docker.com/products/docker-desktop/), then:

```console
$ brew install kind kubectl helm
```

**Linux** — Docker Engine per the [official
instructions](https://docs.docker.com/engine/install/), with your user in the
`docker` group so the script can reach the daemon
(`sudo usermod -aG docker "$USER" && newgrp docker`), then:

```console
$ [ "$(uname -m)" = aarch64 ] && ARCH=arm64 || ARCH=amd64

$ curl -Lo ./kind "https://kind.sigs.k8s.io/dl/latest/kind-linux-${ARCH}"
$ chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind

$ curl -LO "https://dl.k8s.io/release/$(curl -Ls https://dl.k8s.io/release/stable.txt)/bin/linux/${ARCH}/kubectl"
$ chmod +x ./kubectl && sudo mv ./kubectl /usr/local/bin/kubectl

$ curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

**Memory.** Docker needs at least 6 GiB. Docker Desktop hands out 4 GiB by
default, which is not enough for Istio's control plane and Kyverno's webhook on
a single node — raise it under Settings → Resources → Memory. On Linux the
container gets the host's memory, so there is usually nothing to do.

**Ports.** 80 and 443 must be free. The gateway is published on them, so the
URLs below work in a browser with no proxy and no port-forward.

**DNS.** `*.127-0-0-1.sslip.io` is public DNS that resolves to `127.0.0.1`,
which is what keeps this from asking you to edit `/etc/hosts`. If you are
working offline:

```console
$ echo "127.0.0.1 argocd.127-0-0-1.sslip.io demo.127-0-0-1.sslip.io" | sudo tee -a /etc/hosts
```

## Take it for a walk

Seven things worth doing, each about a minute. They are ordered so that every
one builds on the last, and none of them needs anything installed beyond the
prerequisites above.

### 1. Look at the tree

Open `http://argocd.127-0-0-1.sslip.io` — the password is printed at the end of
`make try`, and the user is `admin`.

The `explore-root` Application is an *app-of-apps*: it is an Application whose
job is to create other Applications. Open it and you will find ten of them,
each one a component of the platform. This is the same structure the bare-metal
cluster uses, where the equivalent root manages thirty-eight.

From the terminal:

```console
$ make explore-status
APPLICATION         SYNC     HEALTH
app-demo            Synced   Healthy
explore-resources   Synced   Healthy
explore-root        Synced   Healthy
gateway-api         Synced   Healthy
...
```

### 2. Open the developer portal

This is the part that makes the repository a platform rather than a cluster.

Open `http://backstage.127-0-0-1.sslip.io`, and sign in as a guest — there is no
identity provider here, so Backstage's guest provider is what you get.

Two things are worth finding:

- **Create → the FastAPI template.** This is the golden path: the scaffolder a
  developer uses to start a service that arrives on the platform already wired
  up, rather than assembling manifests by hand.
- **Catalog.** The domains, systems and teams the platform knows about.

Both are read from files inside the image. Production also runs a GitHub
provider that scans the organisation for `catalog-info.yaml` files, and that one
needs a token from Vault — so it is switched off here, and the catalog is
whatever the repository ships. Nothing else about the portal changes.

Backstage is also the only workload in this slice that runs an Istio sidecar,
because its namespace carries `istio-injection: enabled`. `kubectl -n backstage
get pods` shows two containers rather than one.

**Everything you do here disappears.** Production keeps Backstage's catalog in
PostgreSQL with credentials from Vault; a throwaway cluster has neither and
nothing worth persisting, so the explore layer uses the in-memory SQLite the
repository already uses for local development. Restart the pod and the catalog
is rebuilt from the same files.

### 3. Break something and watch it heal

This is the part of GitOps that is hard to appreciate from a diagram. Scale the
demo app by hand:

```console
$ kubectl -n app-demo scale deploy demo --replicas=3
$ kubectl -n app-demo get deploy demo -w
```

Within about ten seconds it is back to one replica. Nobody ran a script; Argo CD
noticed that the cluster no longer matched Git and corrected it. `selfHeal: true`
is set on every Application in this repository, which is why a manual `kubectl
edit` on the real cluster is a temporary opinion rather than a change.

### 4. Follow a request through the Gateway API

The demo app is not port-forwarded. It is reached the way the production apps
are: a `Gateway` that Istio implements, and an `HTTPRoute` that attaches to it.

```console
$ curl -s http://demo.127-0-0-1.sslip.io | head -4
$ kubectl -n istio-system get gateway gateway-explore
$ kubectl -n app-demo get httproute demo -o jsonpath='{.status.parents[0].conditions[*].type}'
Accepted ResolvedRefs
```

`Accepted` means the gateway agreed to serve this route. It can refuse: gateways
in this repository admit routes by namespace label, so a route in a namespace
without `kensan-lab.platform/environment` comes back `NotAllowedByListeners`.
That is a deliberate guardrail — a new application cannot publish itself on the
platform's hostname by accident.

### 5. Watch the policy engine catch you

All six production policies are running, in Audit: they report rather than
block. Start with what is already there:

```console
$ kubectl -n app-demo get policyreport \
    -o custom-columns=PASS:.summary.pass,FAIL:.summary.fail
PASS   FAIL
5      0
```

The demo app passes all five policies that apply to application namespaces —
not because it was written carefully, but because `charts/app-base` sets the
security context, and the values file supplies resource requests. Compare with
the cluster's own components:

```console
$ kubectl -n kube-system get policyreport -o json \
    | jq -r '.items[].results[] | select(.result=="fail") | .policy' | sort -u
pss-baseline
```

kind's control plane uses host namespaces and non-default capabilities, so it
fails the baseline. The engine is genuinely evaluating, not decorating.

Now break a rule on purpose:

```console
$ kubectl -n app-demo run oops --image=nginx:latest --restart=Never
$ sleep 70
$ kubectl -n app-demo get policyreport -o json \
    | jq -r '.items[].results[] | select(.result=="fail") | "\(.policy): \(.message)"'
disallow-latest-tag: validation error: ':latest' tag は禁止です …
require-requests: validation error: 全 container に resources.requests.cpu / memory が必要です …
```

Two policies caught it: the image has a mutable tag, and the pod declares no
resource requests. Nothing was blocked — Audit is the current stage of
[ADR-012](../adr/012-policy-enforcement-kyverno.md), and promoting these to
Enforce is a deliberate later step. Clean up with
`kubectl -n app-demo delete pod oops`.

**Why a minute, and not instantly?** Bare metal scans once an hour and writes
no admission reports, because the etcd write amplification would land on a
microSD card. kind has neither, so the explore layer shortens the interval to
one minute — the only Kyverno setting that differs between the two.

### 6. Claim a volume

Every PVC in this repository asks for `storageClassName: longhorn`, including
the default in `charts/app-base`. There is a StorageClass called `longhorn`
here, and it is not Longhorn:

```console
$ kubectl get storageclass
NAME                 PROVISIONER             VOLUMEBINDINGMODE
longhorn (default)   rancher.io/local-path   WaitForFirstConsumer
standard             rancher.io/local-path   WaitForFirstConsumer
```

Borrowing the name is the entire trick: manifests written for the real cluster
bind here without being edited. Try it:

```console
$ kubectl -n app-demo create -f - <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata: { name: scratch }
spec:
  accessModes: ["ReadWriteOnce"]
  storageClassName: longhorn
  resources: { requests: { storage: 1Gi } }
EOF
$ kubectl -n app-demo get pvc scratch
NAME      STATUS    CAPACITY   STORAGECLASS
scratch   Pending              longhorn
```

`Pending` is correct, not broken. `WaitForFirstConsumer` holds provisioning
until a pod actually needs the volume, so the volume lands on the node the pod
was scheduled to. Give it a consumer and it binds in a few seconds. What you do
*not* get is replication, snapshots or expansion — see below.

### 7. See how Istio got onto the network

Istio's CNI plugin does not replace the cluster's networking; it appends itself
to whatever is already there. On kind that is kindnet:

```console
$ docker exec kensan-lab-explore-control-plane \
    cat /etc/cni/net.d/10-kindnet.conflist | jq -r '[.plugins[].type] | join(" ")'
ptp portmap istio-cni
```

`istio-cni` last, kindnet's own plugins untouched. On bare metal the same
setting chains it onto Cilium instead. Getting this wrong — taking the chain
over rather than joining it — cuts a node off from its own control plane, so
CI asserts this exact output on every pull request.

## How it is put together

A **subset with substitutions**, not a fork. Every component runs the same
Application definition, chart version and values file as bare metal; the
explore layer adds a second Argo CD root beside the production one rather than
copying the tree.

Two things differ, and both are contained in `environments/kind/`:

**What is left out.** No Cilium, Vault, Keycloak, Longhorn, cert-manager,
Cloudflare Tunnel or observability stack. Those need hardware, need credentials
you do not have, or would turn three minutes into thirty.

**What is swapped.** Four files. The Cilium L2 load balancer becomes a kind port
mapping; Longhorn becomes local-path wearing the name `longhorn`; the real
domain and its wildcard certificate become `*.127-0-0-1.sslip.io` over plain
HTTP. Two of the four exist only because a hostname is hardcoded in a raw
manifest, and are scheduled for deletion.

Every pull request that touches `kubernetes/`, `charts/` or `environments/`
stands this cluster up in CI and fails if the platform stops coming up. That is
the real reason the layer exists: it turns "still reproducible from scratch"
from a claim in a README into something a build checks.

The maintenance contract — what is safe to change, what is deliberately
different, and what gets removed later — is in
[`environments/kind/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/environments/kind/README.md).

## What kind cannot show

These are not gaps waiting to be filled. Each is impossible in a way worth
being precise about, and each is a reason the bare-metal cluster is where the
interesting parts live.

**The load balancer.** Cilium announces a virtual IP by answering ARP on the
LAN. kind's nodes are containers on a Docker bridge; there is no physical
segment to announce into and no other machine that could ask. The port mapping
gets traffic to the gateway, but cannot show a VIP failing over between nodes.

**Single sign-on.** Keycloak, oauth2-proxy and the `ext_authz` chain guard every
hostname on the real gateway. Here, Argo CD uses its local admin account and
Backstage its guest provider — both of which production also has, sitting behind
the SSO gate rather than instead of it. Bringing the gate itself up is a later
phase.

**The root of trust for secrets.** This one cannot be solved by more work. The
sealed secrets in this repository are encrypted to *our* controller's key and
are useless to anyone else. Vault's unseal keys and root token come into
existence during `vault operator init` and are never written anywhere they
could be committed; generating them from Terraform would only move the secret
into Terraform state. The irreducible part of bootstrapping a platform is that
somebody has to run a script and keep what it prints.

**Trusted certificates.** The real gateway terminates TLS with a wildcard
certificate issued through a DNS-01 challenge against a domain we own. A fork
has neither. A self-signed issuer is a later phase, and the browser warning
stays — the alternative, `mkcert` rewriting your system trust store, is more
than a one-command demo should do to your laptop.

**Anything about failure.** One node cannot rebuild a Longhorn replica, cannot
be drained, and cannot lose an L2 lease. This cluster shows how the platform is
*composed*; how it *behaves* is told in the [incident
notes](https://github.com/yu-min3/kensan-lab/tree/main/docs/incidents).

**Most of the service mesh data plane.** Backstage runs a sidecar here, so the
mesh is not purely decorative — but production injects sidecars into several
platform namespaces (Vault, Keycloak, oauth2-proxy, External Secrets) and
explore runs none of those. Application namespaces have no sidecar on bare metal
either, so the demo app having a single container is faithful rather than
broken. What you cannot see here is traffic between two meshed services.

**Multiple architectures.** kind's nodes are all your host's architecture, so
the arm64/amd64 split the real cluster schedules around cannot be reproduced.
The applications are built for both, so that is not why they are absent — their
images are private and pulled through Vault-backed credentials.

## Troubleshooting

**A pod is Pending and the node looks full.** Docker has less memory than it
needs. Raise it to 6 GiB or more, then `make explore-down && make try`.

**Ports 80 or 443 are in use.** Find the process with `sudo lsof -nP -iTCP:80
-sTCP:LISTEN`. A previous explore cluster is the usual culprit — `make
explore-down` clears it.

**An Application will not go Healthy.** `explore-up.sh` prints which one it is
waiting for. Most components pass through `Degraded` on the way up and settle by
themselves: sync waves order Application *creation*, not readiness, so the
convergence relies on Argo CD's retry. If one is still stuck after a few
minutes:

```console
$ kubectl -n argocd describe application <name>
```

**The demo app returns 404.** The route was rejected by the listener; see
exercise 4.

```console
$ kubectl -n app-demo get httproute demo -o jsonpath='{.status.parents}' | jq
```

**Everything is healthy but the browser shows nothing.** DNS. Check that
`demo.127-0-0-1.sslip.io` resolves, and use the `/etc/hosts` fallback above if
it does not.

**kind cannot pull the node image.** `environments/kind/kind-cluster.yaml` pins
one by digest, so that everybody gets the Kubernetes version the real cluster
runs rather than whatever their kind release defaults to. A kind old enough not
to support that image will say so; upgrading kind fixes it.

## Where to go next

- [Infrastructure overview](../architecture/infrastructure.md) — what the real
  cluster is made of, and what the eight components you just ran do there
- [Argo CD architecture](../architecture/argocd.md) — the app-of-apps structure
  you watched sync, at production scale
- [ADR-012: policy enforcement with Kyverno](../adr/012-policy-enforcement-kyverno.md)
  — why those policies are in Audit and what promoting them costs
- [Network architecture](../architecture/network.md) — Cilium, the L2 load
  balancer, and the Istio setup this cluster only half reproduces
