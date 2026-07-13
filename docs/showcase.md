# Showcase

This page is the visual entry point for the running kensan-lab platform. The
repository is a reference architecture, but it is also a live system — these
views are the proof.

Four surfaces tell the story: deployment health (Argo CD), observability
(Grafana), the developer platform (Backstage), and zero-trust network
visibility (Hubble) — plus the app the platform exists to run.

## Argo CD — the cluster is GitOps-managed

<figure markdown>
  ![Argo CD applications list — 38 applications Synced and Healthy, 0 OutOfSync, across platform-project and app-project](assets/showcase/argocd-app-tree.png){ width="900" }
  <figcaption>Every platform and app component reconciled by Argo CD — 38 Synced / 0 OutOfSync. A live GitOps system, not static manifests.</figcaption>
</figure>

## Grafana — the operational layer

<figure markdown>
  ![Grafana Cluster Health dashboard — node liveness, uptime, pressure conditions, CPU/memory/disk, and Raspberry Pi CPU temperature](assets/showcase/grafana-cluster-health.png){ width="900" }
  <figcaption>Cluster Health: node liveness and resources across the Pi 5 nodes and the amd64 worker — including the CPU-temperature panel that hardware reality demands.</figcaption>
</figure>

## Backstage — the Internal Developer Platform

<figure markdown>
  ![Backstage entity page for the kensan component — About panel, ownership, tags, a Relations graph, and a TechDocs link](assets/showcase/backstage-catalog.png){ width="900" }
  <figcaption>The kensan component in Backstage: ownership and system, a live Relations graph, and one click to its TechDocs — the catalog that turns "what runs here" into self-service.</figcaption>
</figure>

## Hubble — zero-trust network visibility

<figure markdown>
  ![Hubble UI service map for the app-kensan namespace — flows from the outside world to Syncthing and from the kensan app to the OTel Collector](assets/showcase/hubble-flows.png){ width="900" }
  <figcaption>Cilium/Hubble makes every service-to-service flow observable — here app-kensan's real traffic: Syncthing sync in, telemetry out to the OTel Collector.</figcaption>
</figure>

## kensan — the app that runs on the platform

<figure markdown>
  ![kensan dashboard — North Star and quarterly focus, a today lane, a prioritized backlog, memos, and a whiteboard, rendered in the Whetstone design system](assets/showcase/kensan-app.png){ width="900" }
  <figcaption>Dogfooding: kensan (Go single binary + Whetstone SPA) is a real workload deployed by Argo CD, cataloged in Backstage, and observed like any other service. Shown here with a sample workspace — the app reads Markdown files as its single source of truth.</figcaption>
</figure>

## Capture guidelines

- Use real running screens only. No mock dashboards or generated images.
- Redact domains, user names, tokens, cluster IDs, and IP addresses that are not
  already public.
- Prefer a 16:9 browser viewport around 1440px wide; save as PNG.
- Keep captions outcome-focused: what the screen *proves*, not how to operate the
  tool (the captions above already follow this — adjust only if the shot
  differs).
