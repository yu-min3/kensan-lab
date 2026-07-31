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

# Deliberately docs-site-only. Transclusion shells exist so the reader lands on
# the transcluded README itself; the site home switches its architecture diagram
# with Material's `#only-light` / `#only-dark`, which has no GitHub equivalent
# that also follows the site's own theme toggle (the README uses `<picture>`).
ALLOW: tuple[tuple[str, str, str], ...] = (
    ("docs/architecture/", "snippets", "one-line transclusion of kubernetes/<domain>/README.md"),
    ("docs/index.md", "image", "Material #only-light / #only-dark theme switching"),
)

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


def markdown_files(roots: list[Path]) -> list[Path]:
    out: list[Path] = []
    for root in roots:
        if root.is_file():
            out.append(root)
        else:
            out += [p for p in sorted(root.rglob("*.md")) if SKIP_DIRS.isdisjoint(p.parts)]
    return out


def main(argv: list[str]) -> int:
    repo = Path(__file__).resolve().parent.parent
    roots = [Path(a).resolve() for a in argv] if argv else [
        repo / "README.md", repo / "docs", repo / "kubernetes", repo / "apps"
    ]
    files = markdown_files([r for r in roots if r.exists()])
    findings = [f for p in files for f in check(p, repo)]

    if not findings:
        print(f"GitHub rendering: {len(files)} files clean")
        return 0
    print("These files render on github.com as well as the docs site, and GitHub", file=sys.stderr)
    print("will show the following as literal text:\n", file=sys.stderr)
    for f in findings:
        print(f, file=sys.stderr)
    print(f"\n{len(findings)} finding(s).", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
