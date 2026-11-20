# Vault bootstrap and recovery

Vault serves the cluster from KV v2 through External Secrets Operator (ESO).
Use the workflow that matches the storage you have.

| Vault state | First action | What it preserves |
|---|---|---|
| Running, including a move from Bitwarden | Use the existing operator login and reconcile selected 1Password items | Current auth, KV values, Raft data and recovery quorum |
| Temporarily sealed, storage intact | Restore access to the original AWS KMS key and check Vault health | Current cluster identity and all durable Vault state |
| Storage lost, recoverable backup available | Restore the backup in an isolated recovery environment with the original KMS key | Vault state as of the backup |
| Intentionally building an empty cluster | Initialize once and apply the core and OIDC phases below | Creates a new Vault identity; external credentials must be seeded |

## Secret ownership

Set `KENSAN_OP_VAULT` to the name or ID of a dedicated 1Password vault accessible
to the human operator. Authenticate with the 1Password CLI. Runtime pods use
their existing Vault Kubernetes auth and ESO roles.

| Material | Issuer or owner | Operation |
|---|---|---|
| GitHub READ PAT (Backstage and GHCR) | GitHub issues it; the kensan-lab 1Password Environment holds `READ_GHCR_PAT` | Set `KENSAN_OP_ENVIRONMENT` to the Environment ID and use `sync-github-read.py` for both runtime consumers. The WRITE PAT stays on the build machine. |
| Other provider tokens and static credentials in the sync mapping | Provider issues them; 1Password holds the operator's copy | `sync-op.py` manually reconciles named fields into Vault KV. ESO refreshes the Kubernetes Secret. |
| Keycloak OIDC client secrets | Keycloak | `bootstrap/keycloak/setup.sh` reads or creates the clients and supplies their current values to the consumers. Vault OIDC reads its own client secret from Keycloak. |
| Human Keycloak password | Dedicated 1Password Login `keycloak-user-yu` | Set the password field before creating that user. Existing users keep their current password. |
| Vault emergency userpass password | Dedicated 1Password Login `vault-emergency-admin` | Set the password field before applying the Vault core. Use the matching established password when moving an existing cluster. |
| Initial root token | Vault | Temporary private bootstrap state, revoked at completion. Day-to-day operations use a scoped session. |
| Recovery shares | Vault | A fresh initialization produces five shares, quorum three. The 1Password Document `vault-recovery-<Vault cluster ID>` holds the shares for that cluster. |
| Application-generated values and unmapped KV data | Issuer or Vault | Preserve existing values through a verified backup, or recover from the issuer. The sync mapping enumerates its own coverage. |

## Existing cluster: change the source without reinitializing

1. Identify the active Vault cluster, Kubernetes context, and intended 1Password vault.
   Authenticate to the live Vault using the current OIDC admin session, or use the
   existing emergency userpass login if OIDC is unavailable. Set `VAULT_ADDR`
   explicitly to the intended Vault endpoint. Keep secret values out of logs.
2. While the old custody path is accessible, use the password-manager UI to
   carry forward the **current** emergency password, human password where needed,
   cluster-specific recovery shares and externally issued credentials. Record
   the actual Vault cluster ID and threshold with the shares; keep the initial
   root token out of the recovery Document. If 1Password
   already holds an item, verify the account and field before updating it. Keycloak
   can supply its own OIDC client secrets. A previously revoked root token has no
   use in this migration.
3. From the repository root, list the mapping and compare one group at a time.
   Use the item and field names shown by `--list`. The check compares values
   without printing them or writing to Vault. Resolve unexpected differences
   against the issuer or working system before syncing.

   ```bash
   bootstrap/vault/sync-op.py --list
   bootstrap/vault/sync-op.py --check grafana-admin
   bootstrap/vault/sync-op.py grafana-admin
   ```

   A named sync merges mapped fields with the existing KV v2 record using
   check-and-set. It preserves other fields there. `--all` applies the declared
   groups; use it when all their source items are ready. The shared GitHub READ
   PAT follows a separate path from the already imported kensan-lab Environment.
   Set `KENSAN_OP_ENVIRONMENT` to its ID, compare with
   `bootstrap/vault/sync-github-read.py --check`, then run the same helper
   without `--check` to update both Backstage and GHCR. A later rotation needs
   another sync. For an established database, changing the 1Password field and
   syncing it does not rotate the password inside PostgreSQL. Similarly, the
   Keycloak bootstrap admin setting alone cannot change an existing realm user’s
   password. Verify against the service before selecting a value as authoritative.
4. Check ESO readiness and a real consuming workload, then test both ordinary
   operator access and break-glass access in separate sessions. Record which
   legacy entries still belong to other consumers. For the open-ended Backstage service registry, select the exact integration
   field with `sync-op.py --check backstage-service-credentials --field ENV_KEY`.
   `--all` processes fixed mappings only. Inventory the registry fields and their issuers.
   Keep the existing recovery
   material available until an independently accessible backup has passed a restore
   drill. A live cluster does not need `init.sh` or either Terraform apply for
   this change.

## Empty cluster: establish trust in two phases

The bootstrap Terraform creates Vault's Kubernetes auth roles, emergency
userpass access, policies, KV mount and audit devices. VCO takes over ongoing
policy configuration. Keycloak depends on its database password and bootstrap
admin credentials from Vault via ESO. Configure Vault OIDC **after** Keycloak
starts to avoid making the identity provider a prerequisite for its own secrets.

1. Bring up Raft storage and the Vault HA workloads with access to their AWS
   KMS auto-unseal key. The `vault-aws-kms-credentials` SealedSecret must be
   decryptable by the target cluster's controller. Select the Kubernetes context
   and set `KENSAN_OP_VAULT`. Prepare the emergency login and the mapped
   1Password items for `keycloak-admin` and `keycloak-db`. Initial values for a
   new DB can be generated. A restored database needs its matching credentials.
2. Run `bootstrap/vault/init.sh`. Keep its local state private and available for
   retries. It lives under
   `${XDG_STATE_HOME:-$HOME/.local/state}/kensan-lab/vault-bootstrap/<kube-system namespace UID>`.
   A partial first run should be resumed with that same state and context. An
   already initialized Vault without its matching bootstrap state belongs on the
   existing-cluster or recovery path. If the server initialized but the one-time
   response was lost, preserve the original storage and all partial local files;
   determine what recovery shares were actually captured before proceeding.
3. Start a local port-forward to the intended Vault and keep it running while
   applying. Run the following from the repository root, in separate terminals
   for the port-forward and the operator commands.

   ```bash
   kubectl -n vault port-forward svc/vault 8200:8200
   ```

   ```bash
   export VAULT_ADDR=http://127.0.0.1:8200
   bootstrap/vault/bootstrap-tf.sh core
   vault login -no-print -method=userpass username=emergency-admin
   bootstrap/vault/sync-op.py keycloak-admin
   bootstrap/vault/sync-op.py keycloak-db
   ```

4. Wait for VCO's policies and ESO's target Secrets in `platform-auth-prod`.
   Verify PostgreSQL and Keycloak readiness. Prepare the human Login for a new
   Keycloak user, run `bootstrap/keycloak/setup.sh`, then run
   `bootstrap/vault/bootstrap-tf.sh oidc`. The Vault OIDC client secret is read
   from the issuer by `bootstrap/keycloak/read-vault-client-secret.sh`; the
   helper is called by scripts so its output stays out of the terminal.
5. Verify a fresh OIDC login and the independent emergency login, the expected
   Vault roles and audit devices, an ESO sync, and access to the recovery
   Document for this cluster. Use an authenticated operator session to populate
   other mapped groups as their 1Password items become available. Run
   `bootstrap/vault/bootstrap-tf.sh finish` only after these checks. It revokes
   the temporary root token and cleans up private local bootstrap artifacts.
   Normal restarts use KMS auto-unseal and the existing Raft state.

## Restore the original state

- **KMS unavailable.** Check Vault seal status, the underlying AWS key's identity
  and state, IAM permissions, the Kubernetes credential Secret and network
  reachability. Restore access to the original KMS key. Recovery shares authorize
  operations such as emergency root generation; they cannot decrypt a Vault
  sealed with AWS KMS. Permanent loss of that key makes the old encrypted Raft
  data unusable, even with a snapshot. [HashiCorp seal semantics](https://developer.hashicorp.com/vault/docs/concepts/seal)
- **Raft storage lost.** Use a known-good off-cluster Raft snapshot and the
  supported restore procedure for the Vault version and seal mode. A restore
  replaces destination state. Rehearse it on isolated storage, isolated from
  production consumers. Verify the recovered cluster ID, seal status, an
  authorized login and a consumer read without printing secrets. Check any
  changes issued after the snapshot against the current issuer or 1Password
  item before enabling applications. [HashiCorp snapshot recovery](https://developer.hashicorp.com/vault/docs/sysadmin/snapshots/restore)
- **Backup evidence.** Longhorn schedules volume backups to R2; the storage
  migration runbook separately takes a manual Raft snapshot. A configured job
  or successful upload alone does not demonstrate Vault recovery. Record the
  independently retrievable backup identifier and time, Vault version, KMS key
  identity, and the last successful isolated restore. Also verify access to R2
  and either the original Sealed Secrets controller keys or the ability to
  reissue credentials and reseal them for the replacement cluster.
- **Recovery custody.** Five shares in one 1Password Document provide one
  custody boundary: anyone who can read that Document can satisfy the quorum.
  Store an independently accessible recovery path for the operator account and
  decide who or what should hold quorum if that account is unavailable. The
  1Password Emergency Kit restores access to 1Password; it does not unseal Vault
  or contain a Raft backup.
- **No usable backup.** Use the empty-cluster path only after selecting a fresh
  deployment. It can rebuild configuration and the mapped credentials. Old
  unmapped KV entries, retained DB credentials and Transit ciphertext each need
  their own recovery plan; recreating a Transit key of the same name cannot
  decrypt old ciphertext.

## References

- [Stage 1 infrastructure and KMS prerequisites](https://github.com/yu-min3/kensan-lab/blob/main/docs/bootstrapping/vault-stage1.md)
- [Secret inventory and ownership](https://github.com/yu-min3/kensan-lab/blob/main/docs/secret-management/index.md)
- [Keycloak static database credentials (ADR-019)](https://github.com/yu-min3/kensan-lab/blob/main/docs/adr/019-keycloak-db-credentials-revert-to-static.md)
- [Vault storage migration and backup checks](https://github.com/yu-min3/kensan-lab/blob/main/docs/runbooks/vault-storage-migration-longhorn.md)
