"""Wait for the single Ready application Pod running an expected image."""

import argparse
import json
import subprocess
import sys
import time


def select_pod(items, app, image):
    candidates = []
    for pod in items:
        meta = pod.get("metadata", {})
        if meta.get("labels", {}).get("app.kubernetes.io/name") != app:
            continue
        if meta.get("deletionTimestamp") or not meta.get("name") or not meta.get("uid"):
            continue
        containers = pod.get("spec", {}).get("containers", [])
        if not any(c.get("name") == "app" and c.get("image") == image for c in containers):
            continue
        conditions = pod.get("status", {}).get("conditions", [])
        if any(c.get("type") == "Ready" and c.get("status") == "True" for c in conditions):
            candidates.append((meta["name"], meta["uid"]))
    return candidates[0] if len(candidates) == 1 else None


def kubectl(namespace, *args):
    result = subprocess.run(
        ["kubectl", "-n", namespace, "--request-timeout=10s", *args, "-o", "json"],
        check=True, capture_output=True, text=True, timeout=15,
    )
    return json.loads(result.stdout)


def wait_for_pod(namespace, app, image, timeout):
    deadline = time.monotonic() + timeout
    items = []
    while True:
        try:
            items = kubectl(namespace, "get", "pods", "-l", f"app.kubernetes.io/name={app}")["items"]
            selected = select_pod(items, app, image)
            if selected:
                return selected
        except (subprocess.SubprocessError, ValueError, KeyError) as error:
            print(f"Pod query failed: {error}", file=sys.stderr)
        if time.monotonic() >= deadline:
            break
        time.sleep(min(2, max(0, deadline - time.monotonic())))

    print(f"No unique Ready Pod for {namespace}/{app}, expected image {image}", file=sys.stderr)
    for pod in items:
        meta = pod.get("metadata", {})
        print(json.dumps({
            "name": meta.get("name"), "uid": meta.get("uid"),
            "deletionTimestamp": meta.get("deletionTimestamp"),
            "images": [c.get("image") for c in pod.get("spec", {}).get("containers", [])],
            "conditions": pod.get("status", {}).get("conditions", []),
        }), file=sys.stderr)
    try:
        deployment = kubectl(namespace, "get", "deployment", app)
        print(json.dumps({"generation": deployment["metadata"].get("generation"),
                          "status": deployment.get("status", {})}), file=sys.stderr)
    except (subprocess.SubprocessError, ValueError, KeyError) as error:
        print(f"Deployment diagnostics failed: {error}", file=sys.stderr)
    raise TimeoutError("application Pod selection timed out")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("namespace")
    parser.add_argument("app")
    parser.add_argument("image")
    parser.add_argument("--timeout", type=float, default=180)
    args = parser.parse_args()
    try:
        print(*wait_for_pod(args.namespace, args.app, args.image, args.timeout))
    except TimeoutError as error:
        print(error, file=sys.stderr)
        sys.exit(1)
