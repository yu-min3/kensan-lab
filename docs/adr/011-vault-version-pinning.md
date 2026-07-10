# ADR-011: Vault server image tag explicit pin (chart 0.32.0 + Vault 2.0.0)

## Status

**Accepted** (2026-04-14, adopted as an interim measure)

## Date

2026-04-14

## Context

`kubernetes/secrets/vault/values.yaml` explicitly pins `server.image.tag` to `"2.0.0"`. This ADR records why, and the conditions for removing the pin.

### Background: the silent upgrade accident

In HashiCorp Vault Helm chart `0.32.0` (released 2026-01-14), the template applies `default "latest"` when `server.image.tag` is the empty string `""`, rendering `image: hashicorp/vault:latest`.

This repo had specified the empty string expecting "follow the chart default" — but what it actually selected was `:latest`. When Vault 2.0.0 was published to the `:latest` tag on 2026-04-14, all 3 replicas were silently upgraded to 2.0.0 via `RollingUpdate`. Discovered during review of PR #271.

### Impact

- An unplanned major-version upgrade of Vault (1.x → 2.x)
- Major-version downgrades are officially unsupported (raft snapshot / storage format incompatibility)
- Rolling back would require a manual storage migration — not realistic at homelab scale

## Decision

Explicitly pin `server.image.tag: "2.0.0"`.

```yaml
server:
  image:
    repository: hashicorp/vault
    tag: "2.0.0"
```

### The underlying principle

- Delete the `tag` line and follow the chart default (chart 0.32.0 defaults `server.image.tag` to 1.21.2)
- That keeps chart upgrades and image upgrades in a single review unit

### Why an interim measure instead

- Already running on 2.0.0; downgrading to 1.21 carries raft-storage-format incompatibility risk
- Priority is keeping the running major version stable

## Conditions for removing the pin

Remove the pin once both hold:

1. A chart 0.33+ release exists whose `server.image.tag` chart default has caught up to the 2.x line
2. The running Vault major version and the chart default are the same major at that point

Removal procedure:

- Delete the `server.image.tag` line and follow the chart default
- In the chart-upgrade PR, diff the `helm template` output and check the image tag is the expected 2.x

## Lessons

- Helm charts' `default "latest"` behavior is a breeding ground for silent upgrades
- `tag: ""` can mean "pull `:latest`" rather than "follow the chart default"
- Avoid passing empty strings: either pin explicitly or delete the line

## References

- PR #271: discovery and handling of the silent upgrade
- HashiCorp Vault chart 0.32.0 (released 2026-01-14)
- Vault 2.0.0 (GA 2026-04-14)
