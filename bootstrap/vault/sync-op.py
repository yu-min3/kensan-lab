#!/usr/bin/env python3
"""Publish selected 1Password fields to the KV paths already consumed by ESO."""
import argparse
import os
import re
import subprocess
import sys
from kv_sync import sync_fields


# The source item title is the group name. Fields have the destination labels
# unless the right-hand side names a different 1Password field.
SOURCES = {
    "keycloak-admin": ("platform-auth/prod/keycloak", {"KEYCLOAK_ADMIN_PASSWORD": "password"}),
    "keycloak-db": ("platform-auth/prod/postgresql", {
        "POSTGRES_USER": "username", "POSTGRES_PASSWORD": "password", "POSTGRES_DB": "database",
    }),
    "backstage-service-access": ("backstage/service-access", {"config": "config"}),
    "backstage-db": ("backstage/postgresql", {"POSTGRES_USER": "username", "POSTGRES_PASSWORD": "password"}),
    "cloudflare-tunnel": ("cloudflare-tunnel/token", {"token": "token"}),
    "grafana-admin": ("monitoring/grafana/admin", {"admin-user": "username", "admin-password": "password"}),
    "grafana-cloud": ("monitoring/grafana-cloud/remote-write", {"username": "username", "password": "password"}),
    "alertmanager-slack": ("monitoring/alertmanager/slack", {"webhook-url": "webhook-url"}),
    "feed-google-drive": ("app-kensan/feed-google-drive", {
        "credentials.json": "credentials.json", "output-folder-id": "output-folder-id",
    }),
}

# ESO uses dataFrom.extract here, so operators select the integration's fields explicitly.
REGISTRY = "backstage-service-credentials"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    selection = parser.add_mutually_exclusive_group(required=True)
    selection.add_argument("name", nargs="?", metavar="NAME")
    selection.add_argument("--list", action="store_true", help="show mappings without accessing either store")
    selection.add_argument("--all", action="store_true", help="process every mapped item")
    parser.add_argument("--check", action="store_true", help="compare only; exit 1 on differences")
    parser.add_argument("--field", action="append", default=[], metavar="ENV_KEY",
                        help="select a named field in the Backstage service credential registry")
    args = parser.parse_args()
    if args.list:
        for name, (dest, fields) in SOURCES.items():
            print(f"{name} -> secret/{dest}: "+", ".join(f"{src} -> {dst}" for dst, src in fields.items()))
        print(f"{REGISTRY} -> secret/backstage/service-credentials: --field ENV_KEY (repeatable)")
        return 0
    sources = SOURCES.copy()
    if args.name == REGISTRY:
        if not args.field or not all(re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", f) for f in args.field):
            parser.error("specify --field ENV_KEY for each registry credential to sync")
        sources[REGISTRY] = ("backstage/service-credentials", {f: f for f in args.field})
    elif args.field:
        parser.error(f"--field requires {REGISTRY}")
    if args.name and args.name not in sources:
        parser.error("unknown NAME; see --list")
    op_vault = os.environ.get("KENSAN_OP_VAULT")
    if not op_vault or any(c in op_vault for c in "/?#"):
        parser.error("set KENSAN_OP_VAULT to the 1Password vault name or ID")
    if not os.environ.get("VAULT_ADDR"):
        parser.error("set VAULT_ADDR to the target Vault")
    matches = True
    for name in SOURCES if args.all else [args.name]:
        dest, mapping = sources[name]
        fields = {}
        for dest_field, label in mapping.items():
            ref = f"op://{op_vault}/{name}/{label}"
            val = subprocess.run(["op", "read", "--no-newline", ref], capture_output=True)
            if val.returncode or not val.stdout:
                raise ValueError(f"{name}/{label}: 1Password read failed or empty; stopped.")
            fields[dest_field] = val.stdout.decode()
        if not sync_fields(f"secret/{dest}", fields, args.check):
            matches = False
    return 0 if matches else 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError) as exc:
        sys.exit(str(exc))
