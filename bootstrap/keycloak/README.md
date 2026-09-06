# Keycloak Bootstrap (Stage 1)

A script that creates, in one pass, the OIDC client, user, and groups Keycloak
needs before Vault Stage 1 can come up.

## Why bootstrap at all

Keycloak itself runs under GitOps (Argo CD), but its **realm, groups, user, and
OIDC client** are not declared in Git.

The options were:

- **A**: do it all through the GUI — no reproducibility
- **B**: script `kcadm.sh` (this approach) — one file, reproducible, no dependencies
- **C**: keycloak-config-cli or the Terraform provider — declarative, but a lot of machinery for Stage 1

For what Stage 1 needs — a single OIDC client so Vault can start — B is enough.
Once there are more OIDC clients, C is worth revisiting.

## What it does

1. Creates the realm `kensan`
2. Creates the groups `platform-admin` and `platform-dev`
3. Creates the user `yu` (email: ymisaki00@gmail.com) and assigns it to `platform-admin`
4. Creates the OIDC client `vault`, used by the Vault Stage 1 bootstrap Terraform
5. Stores the client secret and user password in Bitwarden (existing items are updated to the current value; the old value stays in the password history)

## Prerequisites

- `kubectl` context pointing at the kensan-lab cluster
- Keycloak running in the `platform-auth-prod` namespace
- Bitwarden CLI (`bw`) installed, logged in, and unlocked
  ```bash
  export BW_SESSION=$(bw unlock --raw)
  ```
- `jq` and `openssl` installed

## Running it

```bash
chmod +x bootstrap/keycloak/setup.sh
./bootstrap/keycloak/setup.sh
```

It is idempotent: existing Keycloak resources are skipped, and an existing user's
password is left alone. **The client-secret items in Bitwarden are always synced
to Keycloak's current value**, so re-running it also repairs secret drift — after
rebuilding the realm during a disaster recovery, for instance.

> **Exception**: the password for `user-yu` is not synced for an existing user,
> because the script does not reset it on its own. If you reset it by hand, update
> `kensan-lab/keycloak/user-yu` in Bitwarden **by hand** as well.

> **Note**: if the `vault` client's secret is regenerated, Vault's
> `auth/oidc/config` has to be updated separately. The bootstrap Terraform state
> has been discarded, so update it directly with `vault write auth/oidc/config ...`
> — the script prints the exact steps when it runs. (This happened for real: the
> Vault OIDC login outage of 2026-06-06.)

## What it produces

| Where | What |
|------|------|
| Bitwarden `kensan-lab/keycloak/oidc-client-vault` | client_id (`vault`) and client_secret |
| Bitwarden `kensan-lab/keycloak/user-yu` | username (`yu`) and password |
| Keycloak realm `kensan` | the groups, user, and OIDC client, all in place |

## Next step

After this script, bring up Vault Stage 1:

```bash
# 1. confirm Vault has been deployed by Argo CD and its pod is Running
kubectl -n vault get pod

# 2. initialise Vault (once only)
kubectl -n vault exec -it vault-0 -- vault operator init \
  -recovery-shares=5 -recovery-threshold=3
# → prints the root token and the recovery keys
# → store them in Bitwarden:
#     kensan-lab/vault/root-token
#     kensan-lab/vault/recovery-keys

# 3. bootstrap Terraform
cd bootstrap/vault
cp terraform.tfvars.example terraform.tfvars  # fill in the values
terraform init
terraform apply

# 4. discard the state (Pattern A': VCO takes over after bootstrap)
rm -f terraform.tfstate*
```

See `bootstrap/vault/README.md` for the details.

## Starting over

```bash
# delete the realm and re-run setup.sh; Bitwarden is updated to the new values
# (the old values remain in the password history — no need to delete items by hand)
kubectl -n platform-auth-prod exec -it deployment/keycloak -- \
  /opt/keycloak/bin/kcadm.sh delete realms/kensan
```
