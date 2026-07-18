# VCO drift: CRs don't get re-applied after a Vault rebuild

## Symptom

A Vault Config Operator CR (Policy / JWTOIDCAuthEngineRole, etc.) shows `ReconcileSuccessful: True`, but **the corresponding object doesn't actually exist in Vault**.

Concrete example (found 2026-07-06): `JWTOIDCAuthEngineRole/default` shows Successful, but `default` is missing from `vault list auth/oidc/role` → logging in with an empty Role in the Vault UI fails with "claim \"groups\" does not match."

## Cause

VCO does **generation-based reconciliation**. It never re-applies unless the CR's spec changes, so it **never detects state loss on the Vault side** (e.g. a re-init, or a rebuild during disaster recovery). As long as `observedGeneration == generation`, it stays reported as "done."

- Adding an annotation doesn't trigger it either (a metadata-only change doesn't bump generation)

## Recovery procedure

```bash
# 1. Confirm the symptom: the CR is Successful but nothing exists in Vault
kubectl get jwtoidcauthenginerole <name> -n vault -o jsonpath='{.status.conditions}'
# (to check the Vault side: log in via the vault namespace's default SA using k8s auth, then vault list)

# 2. Delete the CR → Argo CD selfHeal recreates it → a fresh reconcile runs as if it were new
kubectl delete jwtoidcauthenginerole <name> -n vault
# (since nothing exists in Vault, the deletion finalizer is a no-op; Argo CD recreates it within seconds)

# 3. Confirm the object was actually recreated
kubectl get jwtoidcauthenginerole <name> -n vault \
  -o jsonpath='{.status.conditions[?(@.type=="ReconcileSuccessful")].status}'
```

## Prevention and notes

- **After rebuilding Vault, sweep every VCO CR — delete each and let selfHeal recreate it** (this should be added as a step in the disaster-recovery procedure)
- `auth/oidc/config` (default_role, etc.) is outside VCO's management (Terraform/manual). It can't be partially updated — changing it requires a full write including the client_secret (see PR #298 / `temp/switch-vault-default-role.sh`)
