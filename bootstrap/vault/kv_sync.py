"""Merge operator-supplied fields into Vault KV v2 without printing their values."""
import json
import os
import subprocess
import sys


def sync_fields(path, fields, check=False):
    if not os.environ.get("VAULT_ADDR"):
        raise ValueError("Set VAULT_ADDR to the intended Vault before syncing.")
    mount, separator, key = path.partition("/")
    if not mount or not separator or not key or any(part in {"", ".", ".."} or part.startswith("-") for part in path.split("/")):
        raise ValueError("Expected a Vault KV v2 path of the form mount/path.")
    if not isinstance(fields, dict) or not fields or not all(
        isinstance(k, str) and k and isinstance(v, str) and v for k, v in fields.items()
    ):
        raise ValueError("Missing or invalid input fields; nothing was written.")

    read = subprocess.run(
        ["vault", "kv", "get", "-format=json", f"-mount={mount}", key],
        capture_output=True,
    )
    if read.returncode == 0:
        record = json.loads(read.stdout)["data"]
        meta = record["metadata"]
        version = meta["version"]
        current = record["data"]
        if not isinstance(current, dict) or meta.get("deletion_time") or meta.get("destroyed"):
            raise ValueError(f"{path}: archived/deleted version; inspect metadata before writing.")
        if all(current.get(k) == v for k, v in fields.items()):
            print(f"{path}: already matches")
            return True
    elif read.returncode == 2 and read.stderr.strip() == f"No value found at {mount}/data/{key}".encode():
        current, version = {}, 0
    else:
        raise ValueError(f"{path}: Vault read failed; verify address, session and permissions.")

    if check:
        print(f"{path}: {'missing' if not version else 'differs'}")
        return False
    result = subprocess.run(
        ["vault", "kv", "put", f"-mount={mount}", f"-cas={version}", key, "-"],
        input=json.dumps({**current, **fields}).encode(),
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    if result.returncode:
        raise ValueError(f"{path}: write failed; check permissions/version and rerun.")
    print(f"{path}: synchronized")
    return True


if __name__ == "__main__":
    try:
        sync_fields(sys.argv[1], json.load(sys.stdin))
    except (OSError, ValueError, KeyError, TypeError) as exc:
        sys.exit(str(exc))
