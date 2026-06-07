# Runbooks

Operational runbooks for recurring tasks and known failure modes on the kensan-lab cluster. Each runbook is a step-by-step procedure you can follow under pressure — what to check, what to run, and how to confirm recovery.

## Index

| Runbook | What it covers |
|---|---|
| [Vault Raft auto-join](vault-raft-join.md) | Rejoining a Vault HA Raft peer after a node restart |
| [Cilium WiFi stability](cilium-wifi-stability.md) | Keeping Cilium healthy on WiFi-attached nodes |
| [Cilium update strategy](cilium-update-strategy.md) | Safely upgrading Cilium without dropping the cluster network |
| [Longhorn restore test](longhorn-restore-test.md) | Verifying Longhorn backups by restoring a volume |
| [ArgoCD repo-server tuning](argocd-repo-server-tuning.md) | Tuning the Argo CD repo-server for large repos |
