# Try it in one command

This is the shortest path through kensan-lab. In about ten minutes you will
inspect a GitOps-managed cluster, open an SSO-protected demo, create a second
application through Backstage, merge its pull request, and watch its CPU usage
change in Grafana.

Pull requests that touch the platform stand this same environment up on CI and
sign in to it, so what follows is checked rather than merely written down.

```mermaid
flowchart LR
    A[make try] --> B[Argo CD<br/>platform is healthy]
    B --> C[Demo app<br/>sign in once]
    C --> D[Backstage<br/>create app2]
    D --> E[Gitea Actions<br/>test and build app2]
    E --> F[Gitea<br/>merge platform PR]
    F --> G[Argo CD<br/>deploys app2 image]
    G --> H[Grafana<br/>watch CPU]
```

The bare-metal cluster is not required. This walkthrough runs a disposable,
single-node kind cluster and binds its gateway only to `127.0.0.1`.

## 1. Start the platform

You need Docker with at least 8 GiB of memory, plus `kind`, `kubectl`, `helm`,
`curl`, and `make`.

=== "macOS"

    Install [Docker Desktop](https://www.docker.com/products/docker-desktop/){ target="_blank" rel="noopener" },
    set **Settings → Resources → Memory** to at least 8 GiB, then install the
    command-line tools:

    ```bash
    brew install kind kubectl helm
    ```

=== "Linux"

    Install [Docker Engine](https://docs.docker.com/engine/install/){ target="_blank" rel="noopener" } and the
    [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation){ target="_blank" rel="noopener" },
    [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/){ target="_blank" rel="noopener" },
    and [Helm](https://helm.sh/docs/intro/install/){ target="_blank" rel="noopener" } CLIs.

Then clone and start the environment:

```bash
git clone https://github.com/yu-min3/kensan-lab
cd kensan-lab
make try
```

`make try` checks the prerequisites, builds the demo and Backstage images from
your checkout, creates the cluster, and waits until every Argo CD Application
is healthy. The initial image build can take several minutes; later runs reuse
Docker's layer cache.

The account used across the platform is fixed for this disposable environment:

| User | Password | Used for |
|---|---|---|
| `demo` | `demo` | Demo apps, Argo CD SSO, Backstage, Grafana SSO, and Gitea |

## 2. Accept the local certificate

Every browser URL uses HTTPS. cert-manager creates a local certificate authority
and signs `*.127-0-0-1.sslip.io`, but your browser does not trust that new CA.
The **Your connection is not private** warning is therefore expected.

For a quick walkthrough, choose **Advanced** and then **Proceed to
…127-0-0-1.sslip.io (unsafe)**. The gateway listens only on localhost.

If you prefer to trust the generated root explicitly, export it before opening
the sites:

```bash
kubectl -n cert-manager get secret explore-ca-tls \
    -o jsonpath='{.data.ca\.crt}' | base64 --decode \
    > /tmp/kensan-lab-explore-ca.crt
```

=== "macOS system trust"

    ```bash
    sudo security add-trusted-cert -d -r trustRoot \
        -k /Library/Keychains/System.keychain \
        /tmp/kensan-lab-explore-ca.crt
    ```

    Remove it when the walkthrough is finished:

    ```bash
    sudo security delete-certificate -c "kensan-lab explore" \
        /Library/Keychains/System.keychain
    ```

=== "Browser trust"

    Open the browser's certificate settings, import
    `/tmp/kensan-lab-explore-ca.crt` under **Authorities**, and allow it to
    identify websites. Remove the `kensan-lab explore` authority afterwards.

Importing the root is optional. The environment never changes the host trust
store automatically.

## 3. See GitOps in Argo CD

Open [Argo CD](https://argocd.127-0-0-1.sslip.io){ target="_blank" rel="noopener" } and choose **Log in via
Keycloak**. Sign in with `demo` / `demo`.

Open `explore-root`. It is the root Application that creates the platform's
other Applications. `app-demo`, `backstage`, `grafana`, and the rest should be
`Synced` and `Healthy`.

Argo CD is the GitOps controller for this cluster. It continuously traces the
`kensan-lab` repository in the in-cluster Gitea, renders the declared Helm and
Kubernetes resources, and reconciles them into the cluster. A green
`Synced / Healthy` application therefore means both that the live resources
match Git and that their workloads are running successfully.

![Argo CD showing explore-root and the Applications it created](assets/argocd-tree.png)

The same state is available from the terminal:

```bash
make explore-status
```

## 4. Open the existing demo

Open the [demo application](https://demo.127-0-0-1.sslip.io){ target="_blank" rel="noopener" }. Keycloak should
reuse the session from Argo CD, so no second password prompt appears.

The app displays its name, theme, greeting, and the identity headers attached by
the gateway. The application itself contains no login implementation: Istio
asks oauth2-proxy to authenticate the request before it reaches the pod.

![The existing demo application after signing in](assets/demo-application.png)

```mermaid
sequenceDiagram
    participant Browser
    participant Gateway as Istio Gateway
    participant Auth as oauth2-proxy + Keycloak
    participant App as demo app
    Browser->>Gateway: HTTPS request
    Gateway->>Auth: Is this session valid?
    Auth-->>Gateway: Yes + user identity
    Gateway->>App: Request + identity headers
    App-->>Browser: Render the demo
```

## 5. Create a different app in Backstage

Open [Backstage](https://backstage.127-0-0-1.sslip.io){ target="_blank" rel="noopener" }, then select **Create →
Golden Path Application**. The Explore catalog has one owner, `demo-team`, so the
walkthrough does not ask you to choose among production teams.

Use these example values:

| Field | Value |
|---|---|
| Application Name | `app2` |
| Description | `Second walkthrough application` |
| Repository | owner `demo`, repository `app2` |
| Theme | `night` |
| Greeting | `Hello from the golden path` |

The name is yours to choose. The rest of this page writes it as `app2`; if you
enter something else, substitute it wherever `app2` appears below.

Review the values and press **Create**. Backstage now:

```mermaid
flowchart TD
    A[Backstage form] --> B[Create app2 repository in local Gitea]
    B --> F[Gitea Actions tests and builds app2]
    F --> G[Push commit-SHA image]
    G --> H[Record image tag in deploy/values.yaml]
    H --> C[Open platform-config PR]
    C --> D[Register app2 in the catalog]
    C --> E[Register the new SSO callback]
    C --> I{You merge the PR}
    I --> J
    J[Argo CD discovers app-app2]
    J --> K[app2 is running]
```

The task stays open while Gitea Actions tests the generated source, builds an
app2-specific image, pushes it, and writes its immutable commit SHA into
`deploy/values.yaml`. A failed build fails the Backstage task and no platform
pull request is created. A completed task means the pull request is safe to
review and merge.

## 6. Merge the local pull request

Follow **Review the kensan-lab pull request** from Backstage, or open
[the `kensan-lab` pull requests in Gitea](https://gitea.127-0-0-1.sslip.io/demo/kensan-lab/pulls){ target="_blank" rel="noopener" }.
Gitea does not share the Keycloak browser session, but it uses the same memorable
credentials: sign in with `demo` / `demo`.

You are the platform administrator for this part of the walkthrough. The new
Platform Config PR should already be waiting in `demo/kensan-lab`. It adds two
files under `environments/kind/generated-applications/app-app2/`; merge it.

![Gitea showing the platform pull request ready for review](assets/gitea-platform-pr.png)

Merging does not deploy from Gitea directly. It updates the Git source of truth;
Argo CD notices that change on its next poll, creates the child Application,
and reconciles the workload. This can take about four minutes after the merge.

```mermaid
flowchart LR
    M[1. Merge PR<br/>in Gitea] --> G[2. main changes<br/>in demo/kensan-lab]
    G -->|Argo CD polls Git| A[3. Argo CD<br/>creates app-NAME]
    A --> K[4. Kubernetes<br/>deploys the SHA image]
    K --> B[5. Open<br/>NAME.127-0-0-1.sslip.io]
```

The generated Argo CD Application and namespace are named `app-<application
name>`; the Deployment and hostname keep the application name itself. With
`app2` from step 5 that is Application and namespace `app-app2`, Deployment
`app2`, and hostname `app2.127-0-0-1.sslip.io`.

Watch the new Argo CD Application appear:

```bash
kubectl -n argocd get application app-app2 -w
```

When it is `Synced` and `Healthy`, open the
[new app2 application](https://app2.127-0-0-1.sslip.io){ target="_blank" rel="noopener" }. Compare it with the
[original demo](https://demo.127-0-0-1.sslip.io){ target="_blank" rel="noopener" }. app2 now runs an image built
from its own repository; its night theme and greeting came from Git-managed
runtime values.

To prove source delivery continues after scaffolding, open
`frontend/src/App.tsx` in the app2 repository, use Gitea's edit button to change
one visible sentence, and commit to `main`. A second Actions run produces a new
SHA tag. Argo CD then replaces the app2 pod; refresh the page to see the code
change. A failed test or build never updates the tag, so the last good pod stays
running.

## 7. Watch CPU rise and fall in Grafana

Open the [Explore App Runtime dashboard](https://grafana.127-0-0-1.sslip.io/d/explore-app-runtime/explore-app-runtime?refresh=10s){ target="_blank" rel="noopener" }
in Grafana. Choose **Sign in with Keycloak** if asked. The dashboard shows one
Deployment's CPU, desired and available replicas, request rate, and request
latency.

It opens on the built-in `demo` application. **Set the Namespace picker to
`app-app2` and the Workload picker to `app2` before going on.** Both pickers are
filled from the cluster, so they offer whatever name you chose in step 5 — but
the panels stay empty until they point at a workload that exists.

In a second terminal, keep one app2 process busy for two minutes:

```bash
kubectl -n app-app2 exec deploy/app2 -- python -c \
    'import time; end=time.time()+120; exec("while time.time() < end: pass")'
```

Prometheus scrapes every 30 seconds. The CPU line rises after one or two scrapes,
then falls again after the command exits. This uses kubelet/cAdvisor metrics;
`metrics-server` and `kubectl top` are not required.

The replica panel also makes the earlier GitOps contract visible: a manual
`kubectl scale` is quickly returned to the Git-declared replica count by Argo
CD, often faster than one Prometheus scrape.

## 8. Clean up

If you imported the local CA, remove it from the trust store first. Then delete
the entire disposable cluster:

```bash
make explore-down
```

## Want the details?

Continue to [How the kind environment works](kind-explained.md) for the component
map, per-application build path, Gateway API and certificate design, optional
Kyverno/storage/CNI exercises, limitations, and troubleshooting.

The [bare-metal bootstrap guide](../bootstrapping/index.md) is a reference for
the live homelab. Its end-to-end clean-room bootstrap is not yet verified;
Ansible and Makefile automation is planned.
