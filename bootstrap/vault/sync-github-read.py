#!/usr/bin/env python3
"""Distribute the shared GitHub READ PAT from the development 1Password Environment."""
import argparse
import base64
import json
import os
import shutil
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request

from kv_sync import sync_fields

CTX = "kubernetes-admin@kubernetes"
KUBE = ["kubectl", "--context", CTX, "--request-timeout=15s"]


def request(url, headers=None):
    req = urllib.request.Request(
        url, headers={"User-Agent": "kensan-token-rotation", **(headers or {})},
    )
    try:
        with urllib.request.urlopen(req, timeout=20) as response:
            raw = response.read()
            return json.loads(raw) if raw else {}, response.headers
    except urllib.error.HTTPError as exc:
        raise RuntimeError(f"HTTP {exc.code}: {url.split('?')[0]}") from None
    except urllib.error.URLError:
        raise RuntimeError("接続に失敗しました") from None


def capture(args, env=None):
    result = subprocess.run(args, env=env, capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError(f"{args[0]} {args[1]} が失敗しました（秘密値保護のため出力省略）")
    return result.stdout.strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="compare both targets without writing")
    args = parser.parse_args()
    for command in ("kubectl", "op", "vault"):
        if not shutil.which(command):
            raise RuntimeError(f"{command} が必要です")
    env_id = os.environ.get("KENSAN_OP_ENVIRONMENT")
    if not env_id:
        raise RuntimeError("Set KENSAN_OP_ENVIRONMENT to the kensan-lab development Environment ID.")
    # Capture one selected value over a pipe. Neither the Vault CLI nor kubectl inherits the Environment.
    env = os.environ.copy()
    env.pop("READ_GHCR_PAT", None)
    pat = capture(["op", "run", "--environment", env_id, "--no-masking", "--",
                   "/bin/sh", "-c", 'test -n "$READ_GHCR_PAT" && printf %s "$READ_GHCR_PAT"'], env=env)
    if not pat:
        raise RuntimeError("READ_GHCR_PAT is missing in the selected Environment")
    user, github_headers = request("https://api.github.com/user", {"Authorization": "Bearer " + pat})
    if user.get("login") != "yu-min3":
        raise RuntimeError("Unexpected GitHub account")
    scopes = {s.strip() for s in github_headers.get("X-OAuth-Scopes", "").split(",")}
    if "read:packages" not in scopes or scopes.intersection({"write:packages", "delete:packages"}):
        raise RuntimeError("Expected a GHCR pull token with read:packages and no package write/delete scope")
    auth = base64.b64encode(("yu-min3:" + pat).encode()).decode()
    for repo in ("yu-min3/kensan", "yu-min3/kensan-lab/backstage"):
        result, _ = request(
            "https://ghcr.io/token?service=ghcr.io&scope=repository:" + repo + ":pull",
            {"Authorization": "Basic " + auth},
        )
        if not result.get("token") and not result.get("access_token"):
            raise RuntimeError("GHCR が取得用トークンを返しませんでした")
    print("GitHub identity and GHCR token issuance verified.")
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    forward = subprocess.Popen(
        KUBE + ["-n", "vault", "port-forward", "--address=127.0.0.1", "svc/vault", f"{port}:8200"],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    try:
        for _ in range(50):
            if forward.poll() is not None:
                raise RuntimeError("Vault port-forward が失敗しました")
            try:
                with socket.create_connection(("127.0.0.1", port), timeout=0.2):
                    break
            except OSError:
                time.sleep(0.2)
        else:
            raise RuntimeError("Vault port-forward がタイムアウトしました")
        os.environ["VAULT_ADDR"] = f"http://127.0.0.1:{port}"
        if forward.poll() is not None:
            raise RuntimeError("Vault port-forward stopped")
        capture(["vault", "token", "lookup", "-format=json"])
        changes = {
            "ghcr/pull-token": {"dockerconfigjson": json.dumps({"auths": {"ghcr.io": {"auth": auth}}})},
            "backstage/github": {"GITHUB_TOKEN": pat},
        }
        matched = [sync_fields("secret/" + path, values, args.check) for path, values in changes.items()]
        if args.check:
            return 0 if all(matched) else 1
        secrets = json.loads(capture(KUBE + ["get", "externalsecrets", "-A", "-o", "json"]))
        targets = []
        for resource in secrets["items"]:
            refs = [d.get("remoteRef", {}).get("key", "") for d in resource.get("spec", {}).get("data", [])]
            if any(ref.removeprefix("secret/") in changes for ref in refs):
                targets.append((resource["metadata"]["namespace"], resource["metadata"]["name"]))
        for ns, name in targets:
            capture(KUBE + ["-n", ns, "annotate", "externalsecret", name,
                            f"force-sync={time.time_ns()}", "--overwrite"])
            print(f"同期要求済み: {ns}/{name}")
        print("Vault 更新と同期要求が完了。Pod の再取得には数分かかることがあります。")
        print("確認: kubectl --context=kubernetes-admin@kubernetes -n app-kensan get pods -w")

    finally:
        forward.terminate()
        try:
            forward.wait(timeout=5)
        except subprocess.TimeoutExpired:
            forward.kill()
            forward.wait()


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (KeyboardInterrupt, EOFError):
        print("\n中断しました。更新済みと表示された項目は反映済みです。", file=sys.stderr)
        sys.exit(1)
    except (RuntimeError, ValueError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        sys.exit(1)
    except Exception:
        print("ERROR: 処理に失敗しました。秘密値保護のため詳細は省略します。更新済み表示を確認してください。", file=sys.stderr)
        sys.exit(1)
