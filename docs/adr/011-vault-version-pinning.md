# ADR-011: Vault server image tag explicit pin (chart 0.32.0 + Vault 2.0.0)

## Status

**Accepted** (2026-04-14, adopted as an interim measure)

## Date

2026-04-14

## Context

`kubernetes/secrets/vault/values.yaml` explicitly pins `server.image.tag` to `"2.0.0"`. This ADR records the background and the conditions for lifting that pin.

### Background: a silent-upgrade incident

The HashiCorp Vault Helm chart `0.32.0` (released 2026-01-14) has a template quirk: when `server.image.tag` is set to an empty string `""`, it falls through to `default "latest"`, rendering `image: hashicorp/vault:latest`.

This repo originally set the empty string expecting to follow the chart default, but `:latest` was actually resolved. When Vault 2.0.0 was published to the `:latest` tag on 2026-04-14, all 3 replicas got silently rolling-upgraded to 2.0.0. This was caught during review of PR #271.

### Impact

- An unplanned major-version upgrade of Vault (1.x → 2.x)
- HashiCorp does not officially support downgrading across major versions (no raft snapshot / storage format compatibility)
- Reverting would require a manual storage migration — not realistic at homelab scale

## Decision

Explicitly pin `server.image.tag: "2.0.0"`.

```yaml
server:
  image:
    repository: hashicorp/vault
    tag: "2.0.0"
```

### The proper principle

- Remove the `tag` line entirely and follow the chart default (chart 0.32.0 defaults `server.image.tag` to `1.21.2`)
- This keeps chart upgrades and image upgrades as a single reviewable unit

### Why an interim measure

- Already running 2.0.0 in production; downgrading to 1.21 carries risk due to incompatible raft storage formats
- Priority is keeping the major version that's currently running stable

## Conditions for lifting the pin

Lift this pin once all of the following are true:

1. A chart 0.33+ line is released whose `server.image.tag` default tracks the 2.x line
2. At that point, the running Vault major version matches the chart default's major version

Removal steps:
- Delete the `server.image.tag` line and let the chart default take over
- In the chart-upgrade PR, diff the `helm template` output to confirm the image tag lands on the expected 2.x line

## Lesson

- A Helm chart's `default "latest"` behavior is fertile ground for silent upgrades
- Passing `tag: ""` does not always mean "follow the chart default" — it can resolve to `:latest`
- Avoid passing an empty string; either pin explicitly or delete the line

## References

- PR #271: discovery and remediation of the silent upgrade
- HashiCorp Vault chart 0.32.0 (released 2026-01-14)
- Vault 2.0.0 (GA 2026-04-14)
