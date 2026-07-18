# Syncthing pairing recovery runbook

The rebuild procedure for when kensan-workspace's local-first sync
(Mac ⇄ cluster, PR #371) loses its config PVC
(`app-kensan/syncthing-config`). The device keys and index DB live only in
this PVC and are never committed to Git (a deliberate trade-off).

## What's lost / what isn't

- Lost: the cluster-side device key (i.e. the pairing with the Mac), folder settings, GUI settings
- Not lost: the **workspace data itself** (a separate PVC, `kensan-workspace`) and the Mac's full copy

## Rebuild procedure (~10 minutes)

1. Recreating the pod generates a new device key. Grab the new ID:
   `kubectl logs -n app-kensan deploy/syncthing | grep "My ID"`
2. Set up the cluster-side folder / LAN-only config (pull the apikey out of config.xml and pass it to the CLI):
   ```
   kubectl exec -n app-kensan deploy/syncthing -- sh -c '
   KEY=$(sed -n "s/.*<apikey>\(.*\)<\/apikey>.*/\1/p" /var/syncthing/config/config.xml)
   stcli() { syncthing cli --gui-address=127.0.0.1:8384 --gui-apikey="$KEY" "$@"; }
   stcli config folders add --id kensan-workspace --label kensan-workspace --path /var/syncthing/data
   stcli config options global-ann-enabled set false
   stcli config options relays-enabled set false
   stcli config options natenabled set false
   stcli config devices add --device-id <Mac's ID> --name mac
   stcli config folders kensan-workspace devices add --device-id <Mac's ID>'
   ```
3. On the Mac, delete the old cluster device and register the new ID:
   ```
   syncthing cli config devices add --device-id <new ID> --name cluster --addresses tcp://192.168.0.245:22000
   syncthing cli config folders kensan-workspace devices add --device-id <new ID>
   ```
4. After the rescan, both sides are nearly identical already, so only the diff transfers. If a conflict shows up, inspect the content and resolve it manually

## Related operational notes

- The GUI (8384) is blocked from in-cluster access too, via the `syncthing-guard` NetworkPolicy. Config changes go through `kubectl exec` only
- `.stignore` isn't synced. Keep both sides' handling of things like `/repositories` aligned manually (see kensan-workspace's `conventions.md`)
- Git operations happen on the Mac only (the cluster side never writes `.git`, by discipline)
- Mac-side Syncthing: managed via brew services (`brew services restart syncthing`)
