# ArgoCD repo-server probe timeout tuning

## Symptom

Right after a Pi 5 worker recovers from NotReady, the Argo CD repo-server drops into CrashLoopBackOff. A burst of chart rendering fires all at once, and the chart default `timeoutSeconds: 1` for the liveness/readiness probes is too short, so the probes get marked as failed.

## Cause

The repo-server handles Helm template rendering and git clones, and a reschedule triggers chart rendering for several Applications in a short window. Against the Pi 5 fleet's CPU capacity, the default 1-second probe timeout is too tight.

## Configuration (current)

`kubernetes/argocd/values.yaml`:

```yaml
repoServer:
  livenessProbe:
    timeoutSeconds: 5
  readinessProbe:
    timeoutSeconds: 5
```

Stretching it to 5 seconds lets probes pass through the load spike. Normal-case detection latency only grows by a few seconds, a minor operational cost.

## Related configuration

`controller.diff.server.side: "true"`: has the K8s API server compute the diff, respecting field-manager ownership. This stops the K8s API / admission webhooks from injecting default values (StatefulSet `dnsPolicy`, HTTPRoute `backendRefs[].kind`, ESO `deletionPolicy`, etc.) that would otherwise be misdetected as drift.

To keep the ESO admission webhook's injected defaults from being misdetected as drift, the following are excluded via `ignoreDifferences`:

```yaml
resource.customizations.ignoreDifferences.external-secrets.io_ExternalSecret: |
  jqPathExpressions:
    - .spec.target.deletionPolicy
    - .spec.data[].remoteRef.conversionStrategy
    - .spec.data[].remoteRef.decodingStrategy
    - .spec.data[].remoteRef.metadataPolicy
    - .spec.data[].remoteRef.nullBytePolicy
```

## Related

- [ArgoCD Keycloak integration](../auth/argocd-keycloak-integration.md)
