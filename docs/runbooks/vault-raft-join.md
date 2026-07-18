# Vault Raft auto-join (why retry_join became mandatory)

## Symptom

After initializing `vault-0`, `vault-1` / `vault-2` stay Sealed and can't join the raft cluster. `vault status` sits at `Initialized: false` indefinitely.

## Cause

Without `retry_join` in the `storage "raft"` block, each Pod never auto-discovers peers. The chart's StatefulSet headless service `vault-internal` is resolvable by name, but without an active join configuration on the raft-protocol side, that alone isn't enough to join the cluster.

This setting was missing until PR #228, so `vault operator raft join` had to be run manually every time.

## Configuration (current)

Include the following in `server.ha.raft.config` in `kubernetes/secrets/vault/values.yaml`:

```hcl
storage "raft" {
  path = "/vault/data"
  retry_join {
    leader_api_addr = "http://vault-0.vault-internal:8200"
  }
  retry_join {
    leader_api_addr = "http://vault-1.vault-internal:8200"
  }
  retry_join {
    leader_api_addr = "http://vault-2.vault-internal:8200"
  }
}
```

Write a `retry_join` entry for all 3 pods. Each pod also attempts to join itself, which is a harmless no-op.

## A chart config gotcha

`server.ha.raft.config` is an HCL string that **fully replaces** the chart default (it can't be deep-merged). When upgrading the chart, check the new default content and confirm nothing required (ui / listener / storage / service_registration) got dropped.

`server.affinity` uses map syntax, so it *is* subject to Helm's deep merge (it ends up overriding the chart default's podAntiAffinity).

## Related

- [Stage 1 Bootstrap](../bootstrapping/12-vault-stage1.md)
- PR #228: added `retry_join`
