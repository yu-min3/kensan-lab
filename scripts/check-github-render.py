#!/usr/bin/env python3
"""Reject Markdown that only renders on the docs site, not on GitHub.

Every `.md` here is read through two engines: MkDocs Material (python-markdown
plus eight extensions) and GitHub (CommonMark plus a sanitiser). Syntax that
belongs to one of those extensions is *not* an error on GitHub — it is passed
through as literal text, so the page keeps building and the break is only
visible to a human who opens the file on github.com.

That is exactly how `docs/showcase.md` shipped: written with `<figure markdown>`
on 2026-07-12, linked from the README in the same commit, and displayed as raw
`![alt](path){ width="900" }` for the next 17 days while `mkdocs build` stayed
green the whole time (PR #468).

`mkdocs build --strict` cannot catch this — the site is the half that works.

Scope is the files that are read *both* ways, because only those carry the
constraint: a page transcluded into the site by `pymdownx.snippets` is also a
README someone opens on github.com. That set is derived from the `--8<--` lines
under `docs/`, so adding a tenth architecture page puts its source in scope
without anyone remembering to update a list here.

Everything else has a single rendering target and is left alone. `docs/**` is
site-only, so Material's extensions are free to be used there. A GitHub-only
file — the top README, `.claude/rules/`, app docs — has no reason to reach for
site syntax in the first place, which is why the scope is the overlap and not
"every Markdown file outside docs/".

Rather than pattern-match the source (the rules for when a raw-HTML block ends
are subtle enough that a regex gets both false positives and false negatives),
this renders each file with markdown-it-py in its CommonMark preset — the same
block grammar GitHub uses — and looks for Markdown syntax that survived into
visible text. If the reader would see it as prose, it is a finding.

Usage:
    check-github-render.py [path ...]        # defaults to the repo's Markdown
"""

from __future__ import annotations

import html
import re
import sys
from pathlib import Path

from markdown_it import MarkdownIt

# Syntax that means an extension did not fire. Matched against rendered text,
# never the source, so a page that *documents* the syntax inside a code fence
# stays clean.
LEAKS: list[tuple[str, re.Pattern[str], str]] = [
    (
        "image",
        re.compile(r"!\[[^\]]*\]\("),
        "an image is showing as its own source — usually a `<figure markdown>` "
        "block with no blank line after the opening tag. Use plain "
        "`<figure><img src=… width=…></figure>`",
    ),
    (
        "attr_list",
        re.compile(r"\{\s*[.#][\w-][^}]*\}|\{\s*(width|height)=[^}]*\}"),
        "a trailing `{ … }` attribute list is showing as text. Put the class on a "
        "wrapper element and style it from `whetstone.css`, or set the attribute "
        "directly on an HTML tag",
    ),
    (
        "admonition",
        re.compile(r"(?<!\S)(?:!!!|\?\?\?\+?)\s+\w"),
        'an admonition is showing as text. Write the extension\'s own output '
        'instead: `<div class="admonition note" markdown>` + blank line + body, '
        "or `<details markdown>` for a collapsible",
    ),
    (
        "content tabs",
        re.compile(r'(?<!\S)===\s+"'),
        "content tabs are showing as text and the bodies have run together. Use a "
        "plain heading per section",
    ),
    (
        "snippets",
        re.compile(r"-{2,3}8<-{2,3}\s"),
        "a snippets include is showing as text",
    ),
]

# Per-file exemptions, as (path prefix, leak kind, reason). Empty today — record
# the next deliberate exception here with its reason rather than widening a
# pattern until it stops catching things.
ALLOW: tuple[tuple[str, str, str], ...] = ()

# pymdownx.snippets accepts one-line includes and a block containing multiple
# paths. One to three dashes are supported by current and legacy syntax.
SNIPPET_MARKER = r"-{1,3}8<-{1,3}"
INCLUDE = re.compile(rf'^{SNIPPET_MARKER}\s+(?:"([^"]+)"|(\S+))\s*$')
INCLUDE_BLOCK = re.compile(rf"^{SNIPPET_MARKER}\s*$")
SKIP_DIRS = {".git", "node_modules", "site", ".venv", ".venv-docs", "temp"}
STRIP = re.compile(r"<(pre|code)\b.*?</\1>|<!--.*?-->|<[^>]+>", re.S)


def visible_text(markdown: str) -> str:
    """Render as GitHub would, then reduce to what a reader actually sees."""
    rendered = MarkdownIt("commonmark").enable("table").render(markdown)
    return html.unescape(STRIP.sub(" ", rendered))


def allowed(rel: str, kind: str) -> bool:
    return any(rel.startswith(p) and kind == k for p, k, _ in ALLOW)


def check(path: Path, repo: Path) -> list[str]:
    rel = (
        path.relative_to(repo).as_posix()
        if path.is_relative_to(repo)
        else path.as_posix()  # explicit path argument from outside the repo
    )
    text = visible_text(path.read_text(encoding="utf-8"))
    findings = []
    for kind, pattern, fix in LEAKS:
        match = pattern.search(text)
        if not match or allowed(rel, kind):
            continue
        findings.append(f"{rel}: {kind} — {match.group().strip()[:60]!r}\n    → {fix}")
    return findings


def resolve_include(repo: Path, spec: str) -> Path | None:
    """Resolve a snippet path, removing an optional line/section suffix."""
    candidate = spec.strip().strip('"')
    while candidate:
        target = repo / candidate
        if target.is_file():
            return target
        if ":" not in candidate:
            return None
        candidate = candidate.rsplit(":", 1)[0]
    return None


def dual_rendered(repo: Path) -> list[Path]:
    """The files the docs site transcludes — the only ones read both ways."""
    found: set[Path] = set()
    for page in (repo / "docs").rglob("*.md"):
        in_block = False
        for line in page.read_text(encoding="utf-8").splitlines():
            stripped = line.strip()
            if INCLUDE_BLOCK.match(stripped):
                in_block = not in_block
                continue
            if in_block:
                if not stripped or stripped.startswith(";"):
                    continue
                target = resolve_include(repo, stripped)
                if target:
                    found.add(target)
                continue
            m = INCLUDE.match(stripped)
            if m:
                target = resolve_include(repo, m.group(1) or m.group(2))
                if target:
                    found.add(target)
    return sorted(found)


def markdown_files(roots: list[Path]) -> list[Path]:
    """Expand explicit files and directories supplied on the command line."""
    files: list[Path] = []
    for root in roots:
        if not root.exists():
            raise FileNotFoundError(root)
        if root.is_file():
            files.append(root)
        else:
            files.extend(
                path
                for path in sorted(root.rglob("*.md"))
                if SKIP_DIRS.isdisjoint(path.parts)
            )
    return files


def main(argv: list[str]) -> int:
    repo = Path(__file__).resolve().parent.parent
    # An explicit path is always checked, so any page can be inspected on demand.
    try:
        files = markdown_files([Path(a).resolve() for a in argv]) if argv else dual_rendered(repo)
    except FileNotFoundError as error:
        print(f"Path does not exist: {error}", file=sys.stderr)
        return 2
    findings = [f for p in files for f in check(p, repo)]

    if not findings:
        scope = "explicit" if argv else "dual-rendered"
        print(f"GitHub rendering: {len(files)} {scope} files clean")
        return 0
    print("These files are rendered by both MkDocs and github.com. GitHub will", file=sys.stderr)
    print("show the following as literal text instead of rendering it:\n", file=sys.stderr)
    for f in findings:
        print(f, file=sys.stderr)
    print(f"\n{len(findings)} finding(s).", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
