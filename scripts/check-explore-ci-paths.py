#!/usr/bin/env python3
"""Derive Explore CI paths from its rendered Applications and bootstrap inputs.

Run --write after adding a shared input. Default mode rejects stale filters.
"""
import argparse
import json
from pathlib import Path
import re
import subprocess

import yaml

ROOT = Path(__file__).resolve().parents[1]
BEGIN = "      # BEGIN generated Explore inputs\n"
END = "      # END generated Explore inputs\n"


def braces(pattern):
    match = re.search(r"\{([^{}]+)\}", pattern)
    if not match:
        return [pattern]
    return [expanded for part in match[1].split(",")
            for expanded in braces(pattern[:match.start()] + part + pattern[match.end():])]


def component_scope(path):
    """Broaden shared inputs to components, keeping cross-component files exact."""
    parts = path.split("/")
    if parts[:2] == ["kubernetes", "backstage"]:
        return "kubernetes/backstage/**"
    if parts[:3] == ["kubernetes", "argocd", "applications"]:
        return str(Path(path).parent) + "/**"
    if parts[:2] == ["kubernetes", "argocd"]:
        # Do not widen values.yaml to argocd/**: that includes every production app.
        return path if len(parts) == 3 else "/".join(parts[:3]) + "/**"
    if parts[0] == "kubernetes" and len(parts) >= 4:
        return "/".join(parts[:3]) + "/**"
    # e.g. observability/namespace.yaml spans several components; no broadening.
    return path


def dependencies(root):
    paths = {"environments/kind/**", "backstage/**", "scripts/explore-*.sh",
             "scripts/explore-*.py", "tests/test_explore*.py", "Makefile",
             "scripts/check-explore-ci-paths.py", ".github/workflows/explore-ci.yml"}
    rendered = subprocess.check_output(
        ["helm", "template", "explore", "environments/kind/applications"], cwd=root, text=True)
    for app in yaml.safe_load_all(rendered):
        if not app:
            continue
        mirror = app.get("metadata", {}).get("annotations", {}).get("kensan-lab.platform/mirrors")
        if mirror:
            paths.add(mirror)
            # ApplicationSet chart versions live in matching generator files.
            mirrored = yaml.safe_load((root / mirror).read_text())
            if mirrored.get("kind") == "ApplicationSet":
                for generator in mirrored["spec"].get("generators", []):
                    for item in generator.get("git", {}).get("files", []):
                        for config in root.glob(item["path"]):
                            if json.loads(config.read_text()).get("name") == app["metadata"]["name"]:
                                paths.add(config.relative_to(root).as_posix())
        spec = app["spec"]
        sources = spec.get("sources", [spec.get("source", {})])
        for source in sources:
            for value in source.get("helm", {}).get("valueFiles", []):
                if value.startswith("$values/"):
                    paths.add(value.removeprefix("$values/"))
            path = source.get("path")
            if not path:
                continue
            directory = source.get("directory", {})
            include = directory.get("include")
            if include and not directory.get("recurse", False):
                paths.update(f"{path}/{item}" for item in braces(include))
            else:
                # Covers chart helpers, nested charts, policies and new files.
                paths.add(f"{path}/**")
    bootstrap = (root / "scripts/explore-up.sh").read_text()
    versions = (root / "environments/kind/versions.sh").read_text()
    paths.update(re.findall(r"^# mirrors: (\S+)", versions, re.MULTILINE))
    for path in re.findall(r'\$\{REPO_ROOT\}/([^"\s]+)', bootstrap):
        if "$" in path:
            raise ValueError(f"unresolved bootstrap input: {path}")
        paths.add(path.rstrip("/") + "/**" if (root / path).is_dir() else path)
    # These directories are already covered by broader runtime input scopes.
    paths = {p for p in paths if not p.startswith(("environments/kind/", "backstage/"))} | {
        "environments/kind/**", "backstage/**"}
    paths = {component_scope(p) for p in paths}
    paths = {p for p in paths if not any(
        other != p and other.endswith("/**") and p.startswith(other[:-2])
        for other in paths)}
    return sorted(paths) + ["!kubernetes/**/*.md", "!charts/**/*.md", "!environments/**/*.md"]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true")
    args = parser.parse_args()
    workflow = ROOT / ".github/workflows/explore-ci.yml"
    text = workflow.read_text()
    block = BEGIN + "".join(f"      - {json.dumps(p)}\n" for p in dependencies(ROOT)) + END
    updated, count = re.subn(re.escape(BEGIN) + r".*?" + re.escape(END), lambda _: block,
                             text, flags=re.DOTALL)
    if count != 2:
        raise SystemExit("expected generated input blocks for both pull_request and push")
    if args.write:
        workflow.write_text(updated)
    elif text != updated:
        raise SystemExit("Explore CI inputs changed; run python3 scripts/check-explore-ci-paths.py --write")
    print("Explore CI input filters are up to date")


if __name__ == "__main__":
    main()
