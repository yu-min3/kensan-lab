#!/usr/bin/env python3
"""Reject pages that reach the site but never reach `llms.txt`.

The site publishes two views of the same `docs/` tree. `nav` in `mkdocs.yml` is
the human one — five tabs, nested by what a reader is trying to do. The
`llmstxt` plugin's `sections` is the machine one — a flat topical index at
`/llms.txt`, with the full text at `/llms-full.txt`.

The two are written separately, and they are *meant* to differ in shape: a
reader wants Architecture and Decisions in one tab, and a model retrieving from
this repository is better served by a flat list. What they are not meant to
differ in is coverage. Every page on the site should appear in exactly one
section of the index.

Nothing enforces that today. `llmstxt` matches its sections by glob, and a page
matched by no glob is simply absent from the output. `mkdocs build --strict`
stays green, because from MkDocs' point of view nothing is wrong — the page
built, and it is in the nav. Only somebody diffing 76 nav entries against 76
`llms.txt` entries would notice, and nobody does that.

That failure has a shape: add `docs/newthing/foo.md`, put it in the nav, and the
site is complete while `/llms.txt` silently drops it. This check closes that
window by asserting the same coverage the nav has.

Usage:
    check-llms-coverage.py [mkdocs.yml]
"""

from __future__ import annotations

import fnmatch
import sys
from pathlib import Path

import yaml


def _plain_loader() -> type[yaml.SafeLoader]:
    """A SafeLoader that tolerates mkdocs.yml's `!!python/name:` tags.

    Those tags are how Material wires in its emoji and superfences handlers.
    They are irrelevant here — this only needs `docs_dir`, `exclude_docs`, and
    the llmstxt sections — so they are read as opaque strings rather than
    resolved.
    """

    class Loader(yaml.SafeLoader):
        pass

    Loader.add_multi_constructor(
        "tag:yaml.org,2002:python/name:", lambda loader, suffix, node: suffix
    )
    Loader.add_multi_constructor("!!python/name:", lambda loader, suffix, node: suffix)
    return Loader


def llms_sections(config: dict) -> dict[str, list[str]]:
    """The `sections` mapping from the llmstxt plugin entry, or {} if absent."""
    for plugin in config.get("plugins") or []:
        if isinstance(plugin, dict) and "llmstxt" in plugin:
            return (plugin["llmstxt"] or {}).get("sections") or {}
    return {}


def excluded(patterns: str | None) -> list[str]:
    """`exclude_docs` is a newline-separated gitignore-ish block."""
    if not patterns:
        return []
    return [line.strip() for line in patterns.splitlines() if line.strip()]


def site_pages(docs_dir: Path, exclusions: list[str]) -> list[str]:
    pages = []
    for path in sorted(docs_dir.rglob("*.md")):
        rel = path.relative_to(docs_dir).as_posix()
        if any(rel.startswith(e.rstrip("/") + "/") or fnmatch.fnmatch(rel, e) for e in exclusions):
            continue
        pages.append(rel)
    return pages


def matched(page: str, globs: list[str]) -> bool:
    # `architecture/*.md` is expected to cover `architecture/network/index.md`
    # too — the plugin's own matching is recursive, so `*` is treated as
    # crossing directory boundaries here as well.
    return any(fnmatch.fnmatch(page, g) for g in globs)


def main(argv: list[str]) -> int:
    repo = Path(__file__).resolve().parent.parent
    config_path = Path(argv[0]).resolve() if argv else repo / "mkdocs.yml"
    if not config_path.exists():
        print(f"No such config: {config_path}", file=sys.stderr)
        return 2

    config = yaml.load(config_path.read_text(encoding="utf-8"), Loader=_plain_loader())
    sections = llms_sections(config)
    if not sections:
        print("No llmstxt sections configured — nothing to check.")
        return 0

    docs_dir = config_path.parent / (config.get("docs_dir") or "docs")
    pages = site_pages(docs_dir, excluded(config.get("exclude_docs")))

    every_glob = [g for globs in sections.values() for g in globs]
    missing = [p for p in pages if not matched(p, every_glob)]

    # A page in two sections is not fatal, but it does mean the index shows it
    # twice, which is worth knowing about.
    duplicated = [
        p
        for p in pages
        if sum(1 for globs in sections.values() if matched(p, globs)) > 1
    ]

    if not missing and not duplicated:
        print(f"llms.txt coverage: {len(pages)} pages, all in exactly one section")
        return 0

    if missing:
        print("These pages build into the site but match no llmstxt section, so", file=sys.stderr)
        print("they are absent from /llms.txt and /llms-full.txt:\n", file=sys.stderr)
        for page in missing:
            print(f"  {page}", file=sys.stderr)
        print("\nAdd them to plugins.llmstxt.sections in mkdocs.yml.", file=sys.stderr)
    if duplicated:
        print("\nThese pages match more than one llmstxt section and will be", file=sys.stderr)
        print("listed twice:\n", file=sys.stderr)
        for page in duplicated:
            print(f"  {page}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
