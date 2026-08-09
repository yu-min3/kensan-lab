#!/usr/bin/env python3
"""Argo CD Application / ApplicationSet の静的検証。

- source path がリポジトリに実在するか（repoURL がこのリポジトリのもののみ）
- Application 名の重複がないか（同名 takeover → cascade prune 事故の再発防止）
- ApplicationSet git generator のパターンが 1 件以上マッチするか
- explore (environments/kind) の Application が本番の chart version から drift
  していないか（下記 mirror check）

## mirror check

kind explore 層は本番の Application を複製せざるを得ない（kustomize の既定 load
restriction で共有 app.yaml を外部参照できず、共有ツリーに kustomization.yaml を
置くと platform-root が Kustomization を manifest として apply しようとする）。
そこで複製そのものは許し、**ズレたら CI で落ちる**形に変換する。

explore 側の Application に

    metadata.annotations["kensan-lab.platform/mirrors"]: <本番 app.yaml のパス>

を付けると、両者の upstream chart（chart 名 / repoURL / targetRevision）が一致
することを検証する。silent drift を CI failure に変える 1 点だけが目的で、values
やコンポーネントの取捨選択は対象外（explore は subset なので当然ズレる）。
"""

import glob
import os
import re
import sys

import yaml

REPO_URL = "https://github.com/yu-min3/kensan-lab"
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# Application 名は「1 クラスタ内で一意」であればよい。explore と本番は同時に存在
# しないので、名前の重複検査はツリー単位で行う
TREES = {
    "bare-metal": "kubernetes/argocd/**/*.yaml",
    "explore": "environments/kind/**/*.yaml",
}

MIRROR_ANNOTATION = "kensan-lab.platform/mirrors"

errors: list[str] = []
app_names: dict[tuple[str, str], str] = {}


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


# explore の Application は Helm chart の template として置かれている
# (environments/kind/applications/templates/)。差し込まれるのは repoURL /
# targetRevision の 2 つだけなので、mirror check に必要な範囲では Go template を
# 剥がして YAML として読める。値そのものは検証対象ではない（本番と一致するのは
# chart 側だけで、git source の revision は explore-root が注入する）
TEMPLATE_LINE = re.compile(r"^\s*\{\{-?\s")
TEMPLATE_EXPR = re.compile(r"\{\{.*?\}\}")


def load_docs(path: str) -> list:
    with open(path) as fh:
        text = fh.read()
    if "{{" in text:
        kept = [ln for ln in text.splitlines() if not TEMPLATE_LINE.match(ln)]
        text = TEMPLATE_EXPR.sub("templated", "\n".join(kept))
    return list(yaml.safe_load_all(text))


def read_app(rel: str) -> dict | None:
    """本番 app.yaml を 1 つ読む（mirror 先の解決用）。"""
    path = os.path.join(ROOT, rel)
    if not os.path.isfile(path):
        return None
    for doc in load_docs(path):
        if isinstance(doc, dict) and doc.get("kind") == "Application":
            return doc
    return None


def chart_identity(spec: dict) -> dict:
    """upstream chart source を {chart 名: (repoURL, targetRevision)} で返す。"""
    return {
        src["chart"]: (src.get("repoURL"), str(src.get("targetRevision")))
        for src in iter_sources(spec)
        if src.get("chart")
    }


def check_mirror(rel: str, doc: dict) -> None:
    target = doc.get("metadata", {}).get("annotations", {}).get(MIRROR_ANNOTATION)
    if not target:
        return
    mirrored = read_app(target)
    if mirrored is None:
        errors.append(f"{rel}: {MIRROR_ANNOTATION} の参照先に Application が無い: {target}")
        return

    here = chart_identity(doc.get("spec", {}))
    there = chart_identity(mirrored.get("spec", {}))
    for chart, identity in here.items():
        if chart not in there:
            errors.append(f"{rel}: chart '{chart}' が mirror 元 {target} に存在しない")
        elif there[chart] != identity:
            errors.append(
                f"{rel}: chart '{chart}' が mirror 元から drift — "
                f"explore {identity} / {target} {there[chart]}"
            )

    here_paths = {s["path"] for s in iter_sources(doc.get("spec", {})) if s.get("path")}
    there_paths = {s["path"] for s in iter_sources(mirrored.get("spec", {})) if s.get("path")}
    for path in sorted(here_paths - there_paths - {"environments/kind/resources"}):
        print(f"WARN {rel}: git path '{path}' は mirror 元 {target} に無い（意図的なら無視）")


def check_file(tree: str, path: str) -> None:
    rel = os.path.relpath(path, ROOT)
    try:
        docs = load_docs(path)
    except yaml.YAMLError as e:
        errors.append(f"{rel}: YAML parse error: {e}")
        return

    for doc in docs:
        if not isinstance(doc, dict):
            continue
        kind = doc.get("kind")

        if kind == "Application":
            name = doc.get("metadata", {}).get("name")
            key = (tree, name)
            if name and key in app_names:
                errors.append(f"{rel}: Application 名 '{name}' が {app_names[key]} と重複")
            elif name:
                app_names[key] = rel
            for src in iter_sources(doc.get("spec", {})):
                check_source(src, rel)
            check_mirror(rel, doc)

        elif kind == "ApplicationSet":
            spec = doc.get("spec", {})
            for src in iter_sources(spec.get("template", {}).get("spec", {})):
                check_source(src, rel)
            for gen in spec.get("generators") or []:
                git = gen.get("git") or {}
                for field in ("files", "directories"):
                    for item in git.get(field) or []:
                        pattern = item.get("path", "")
                        if not pattern or "{{" in pattern:
                            continue
                        if not glob.glob(os.path.join(ROOT, pattern), recursive=True):
                            # インスタンス 0 件は合法 (全撤去後など)。ディレクトリ自体が
                            # 無い場合のみ typo / 移動漏れとして fail する
                            dir_pattern = os.path.dirname(pattern)
                            if dir_pattern and glob.glob(os.path.join(ROOT, dir_pattern), recursive=True):
                                print(f"WARN {rel}: generator パターンが 0 件マッチ (ディレクトリは存在・空): {pattern}")
                            else:
                                errors.append(f"{rel}: git generator パターンのディレクトリが存在しない: {pattern}")


def check_versions_env() -> None:
    """explore が Argo CD 本体を helm で入れるときの chart version を検証する。

    この 1 つだけは Application に書けない（Argo CD が存在する前に必要なため）。
    versions.sh を唯一の置き場にして、本番の gitops/argocd app.yaml と一致する
    ことをここで担保する。
    """
    env_path = os.path.join(ROOT, "environments/kind/versions.sh")
    if not os.path.isfile(env_path):
        return
    with open(env_path) as fh:
        declared = dict(
            line.strip().split("=", 1)
            for line in fh
            if "=" in line and not line.lstrip().startswith("#")
        )

    mirrored = read_app("kubernetes/argocd/applications/gitops/argocd/app.yaml")
    if mirrored is None:
        errors.append("environments/kind/versions.sh: 本番の argocd Application が見つからない")
        return
    expected = chart_identity(mirrored.get("spec", {})).get("argo-cd")
    if expected is None:
        errors.append("environments/kind/versions.sh: 本番の argocd Application に argo-cd chart が無い")
        return
    repo, version = expected
    if declared.get("ARGOCD_CHART_VERSION") != version:
        errors.append(
            f"environments/kind/versions.sh: ARGOCD_CHART_VERSION が drift — "
            f"{declared.get('ARGOCD_CHART_VERSION')} / 本番 {version}"
        )
    if declared.get("ARGOCD_CHART_REPO") != repo:
        errors.append(
            f"environments/kind/versions.sh: ARGOCD_CHART_REPO が drift — "
            f"{declared.get('ARGOCD_CHART_REPO')} / 本番 {repo}"
        )


for tree_name, tree_pattern in TREES.items():
    for f in sorted(glob.glob(os.path.join(ROOT, tree_pattern), recursive=True)):
        check_file(tree_name, f)

check_versions_env()

if errors:
    print("NG:")
    for e in errors:
        print(" -", e)
    sys.exit(1)

print(f"OK: Application {len(app_names)} 件 — source path / 名前重複 / generator パターン検証済み")
