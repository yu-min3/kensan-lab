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

_Screenshot pending._
<!-- Save to: docs/assets/showcase/backstage-catalog.png · then replace the line above with:
<figure markdown>
  ![Backstage Software Catalog listing kensan-lab components with TechDocs links](assets/showcase/backstage-catalog.png){ width="900" }
  <figcaption>Service catalog, Golden Path templates, and TechDocs — self-service for app developers.</figcaption>
</figure>
-->

## Hubble — zero-trust network visibility

<figure markdown>
  ![Hubble UI service map for the app-kensan namespace — flows from the outside world to Syncthing and from the kensan app to the OTel Collector](assets/showcase/hubble-flows.png){ width="900" }
  <figcaption>Cilium/Hubble makes every service-to-service flow observable — here app-kensan's real traffic: Syncthing sync in, telemetry out to the OTel Collector.</figcaption>
</figure>

## kensan — the app that runs on the platform

_Screenshot pending._
<!-- Save to: docs/assets/showcase/kensan-app.png · then replace the line above with:
<figure markdown>
  ![kensan app UI — file-based knowledge & goal manager running on the platform](assets/showcase/kensan-app.png){ width="900" }
  <figcaption>Dogfooding: kensan (Go + Whetstone SPA) is a real workload deployed by Argo CD, cataloged in Backstage, and observed like any other service.</figcaption>
</figure>
-->

## Capture guidelines

- Use real running screens only. No mock dashboards or generated images.
- Redact domains, user names, tokens, cluster IDs, and IP addresses that are not
  already public.
- Prefer a 16:9 browser viewport around 1440px wide; save as PNG.
- Keep captions outcome-focused: what the screen *proves*, not how to operate the
  tool (the captions above already follow this — adjust only if the shot
  differs).
