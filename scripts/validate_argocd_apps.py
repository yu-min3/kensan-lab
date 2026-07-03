#!/usr/bin/env python3
"""Argo CD Application / ApplicationSet の静的検証。

- source path がリポジトリに実在するか（repoURL がこのリポジトリのもののみ）
- Application 名の重複がないか（同名 takeover → cascade prune 事故の再発防止）
- ApplicationSet git generator のパターンが 1 件以上マッチするか
"""

import glob
import os
import sys

import yaml

REPO_URL = "https://github.com/yu-min3/kensan-lab"
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

errors: list[str] = []
app_names: dict[str, str] = {}


def check_source(src: dict, origin: str) -> None:
    repo = src.get("repoURL", "").rstrip("/").removesuffix(".git")
    if repo != REPO_URL:
        return  # upstream chart 等は対象外
    path = src.get("path")
    if not path or "{{" in path:
        return  # ref 専用 source / テンプレート変数はスキップ
    if not os.path.isdir(os.path.join(ROOT, path)):
        errors.append(f"{origin}: source path が存在しない: {path}")


def iter_sources(spec: dict):
    yield from spec.get("sources") or []
    if "source" in spec:
        yield spec["source"]


for f in sorted(glob.glob(os.path.join(ROOT, "kubernetes/argocd/**/*.yaml"), recursive=True)):
    rel = os.path.relpath(f, ROOT)
    try:
        with open(f) as fh:
            docs = list(yaml.safe_load_all(fh))
    except yaml.YAMLError as e:
        errors.append(f"{rel}: YAML parse error: {e}")
        continue

    for doc in docs:
        if not isinstance(doc, dict):
            continue
        kind = doc.get("kind")

        if kind == "Application":
            name = doc.get("metadata", {}).get("name")
            if name and name in app_names:
                errors.append(f"{rel}: Application 名 '{name}' が {app_names[name]} と重複")
            elif name:
                app_names[name] = rel
            for src in iter_sources(doc.get("spec", {})):
                check_source(src, rel)

        elif kind == "ApplicationSet":
            spec = doc.get("spec", {})
            for src in iter_sources(spec.get("template", {}).get("spec", {})):
                check_source(src, rel)
            for gen in spec.get("generators") or []:
                git = gen.get("git") or {}
                for key in ("files", "directories"):
                    for item in git.get(key) or []:
                        pattern = item.get("path", "")
                        if not pattern or "{{" in pattern:
                            continue
                        if not glob.glob(os.path.join(ROOT, pattern), recursive=True):
                            errors.append(f"{rel}: git generator パターンが 0 件マッチ: {pattern}")

if errors:
    print("NG:")
    for e in errors:
        print(" -", e)
    sys.exit(1)

print(f"OK: Application {len(app_names)} 件 — source path / 名前重複 / generator パターン検証済み")
