#!/usr/bin/env python3
"""Generate the light-theme draw.io diagram from the dark one, then verify it.

The dark file is the only thing edited by hand; light is a build artifact. Two
contracts keep that safe:

  1. fail-closed — every colour literal in the source must be a diagram token,
     so a box added with an off-palette colour breaks the build instead of
     silently shipping a light diagram that still has a dark box in it;
  2. contrast audit — the generated file is checked against WCAG AA (4.5 for
     text, 3.0 for strokes) before it is written, so a bad token mapping is
     caught here rather than in review.

Usage:
    diagram-theme.py generate <dark.drawio> <light.drawio>
    diagram-theme.py audit <file.drawio>            # audit only, either mode
"""

from __future__ import annotations

import argparse
import json
import re
import sys
import xml.etree.ElementTree as ET
from html.parser import HTMLParser
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
TOKENS = REPO_ROOT / "packages" / "design-tokens" / "tokens.json"

HEX_RE = re.compile(r"#[0-9A-Fa-f]{6}")
# `color:` only — not background-color / border-color.
STYLE_COLOR_RE = re.compile(r"(?:^|;)\s*color\s*:\s*(#[0-9A-Fa-f]{6})", re.IGNORECASE)
TEXT_AA = 4.5
STROKE_AA = 3.0
DEFAULT_FONT_COLOR = "#000000"  # what draw.io paints text with when unstyled

# Text is checked at 4.5 rather than the large-text 3.0 allowance: the diagram's
# body copy is 12–13px, and the few large labels are not worth a second rule.


def load_mapping() -> dict[str, str]:
    """dark hex -> light hex, keyed by the role names in tokens.json."""
    tokens = json.loads(TOKENS.read_text())
    try:
        dark = tokens["diagram"]["dark"]
        light = tokens["diagram"]["light"]
    except KeyError as exc:
        sys.exit(f"tokens.json has no diagram section ({exc})")

    roles = [r for r in dark if not r.startswith("_")]
    missing = [r for r in roles if r not in light]
    if missing:
        sys.exit(f"diagram tokens: roles missing a light value: {', '.join(missing)}")

    mapping: dict[str, str] = {}
    for role in roles:
        src = dark[role]["hex"].upper()
        dst = light[role]["hex"].upper()
        if src in mapping and mapping[src] != dst:
            sys.exit(
                f"diagram tokens: dark {src} maps to both {mapping[src]} and {dst} "
                f"(role '{role}') — the mapping has to stay 1:1"
            )
        mapping[src] = dst
    return mapping


def replace_colors(text: str, mapping: dict[str, str]) -> str:
    """Rewrite every colour literal, refusing to run if one is not a token."""
    unknown: dict[str, int] = {}
    for line_no, line in enumerate(text.splitlines(), start=1):
        for hex_value in HEX_RE.findall(line):
            if hex_value.upper() not in mapping:
                unknown.setdefault(hex_value.upper(), line_no)

    if unknown:
        detail = "\n".join(
            f"  {hex_value}  (first seen on line {line})"
            for hex_value, line in sorted(unknown.items())
        )
        sys.exit(
            "diagram-theme: colours with no diagram token — add them to "
            f"packages/design-tokens/tokens.json first:\n{detail}"
        )

    return HEX_RE.sub(lambda m: mapping[m.group(0).upper()], text)


# --- contrast ---------------------------------------------------------------


def relative_luminance(hex_value: str) -> float:
    channels = []
    for i in (1, 3, 5):
        c = int(hex_value[i : i + 2], 16) / 255
        channels.append(c / 12.92 if c <= 0.03928 else ((c + 0.055) / 1.055) ** 2.4)
    r, g, b = channels
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def contrast(fg: str, bg: str) -> float:
    a, b = relative_luminance(fg), relative_luminance(bg)
    lighter, darker = max(a, b), min(a, b)
    return (lighter + 0.05) / (darker + 0.05)


def parse_style(style: str | None) -> dict[str, str]:
    out: dict[str, str] = {}
    for part in (style or "").split(";"):
        if "=" in part:
            key, _, value = part.partition("=")
            out[key.strip()] = value.strip()
    return out


class LabelRuns(HTMLParser):
    """Split a draw.io HTML label into (text, colour) runs.

    Labels carry their colour as `<font color>` / `<span style="...color:#X">`
    rather than `<b>`, because draw.io drops the shape's fontColor inside a `<b>`
    and paints the text black. Reading runs rather than scraping every hex means
    each piece of text is checked against the colour actually painting it.
    """

    def __init__(self, base_color: str | None):
        super().__init__(convert_charrefs=True)
        self.stack: list[str | None] = [base_color]
        self.runs: list[tuple[str, str]] = []

    def _color_of(self, attrs: list[tuple[str, str | None]]) -> str | None:
        attr = dict(attrs)
        color = attr.get("color") or ""
        if HEX_RE.fullmatch(color):
            return color.upper()
        match = STYLE_COLOR_RE.search(attr.get("style") or "")
        return match.group(1).upper() if match else None

    def handle_starttag(self, tag, attrs):
        if tag == "br":
            return
        self.stack.append(self._color_of(attrs) or self.stack[-1])

    def handle_endtag(self, tag):
        if tag != "br" and len(self.stack) > 1:
            self.stack.pop()

    def handle_data(self, data):
        text = data.replace("\xa0", " ").strip()
        if text and self.stack[-1]:
            self.runs.append((text, self.stack[-1]))


def label_runs(value: str | None, base_color: str | None) -> list[tuple[str, str]]:
    if not (value or "").strip():
        return []
    parser = LabelRuns(base_color)
    parser.feed(value)
    return parser.runs or ([(value.strip(), base_color)] if base_color else [])


class Audit:
    def __init__(self, path: Path):
        self.path = path
        root = ET.parse(path).getroot()
        model = root.find(".//mxGraphModel")
        if model is None:
            sys.exit(f"{path}: no mxGraphModel")
        self.page_bg = (model.get("background") or "#FFFFFF").upper()
        self.cells = {
            cell.get("id"): cell for cell in model.iter("mxCell") if cell.get("id")
        }
        self.failures: list[str] = []

    def backdrop(self, cell: ET.Element | None) -> str:
        """The colour a cell's own text sits on."""
        if cell is None:
            return self.page_bg
        fill = parse_style(cell.get("style")).get("fillColor")
        if fill and fill.lower() != "none":
            return fill.upper()
        return self.backdrop(self.cells.get(cell.get("parent")))

    def parent_backdrop(self, cell: ET.Element) -> str:
        """The colour a cell's stroke sits on."""
        return self.backdrop(self.cells.get(cell.get("parent")))

    def check(self, fg: str, bg: str, minimum: float, what: str) -> None:
        ratio = contrast(fg, bg)
        if ratio < minimum:
            self.failures.append(
                f"{what}: {fg} on {bg} = {ratio:.2f} (needs {minimum})"
            )

    def run(self) -> list[str]:
        for cell_id, cell in self.cells.items():
            style = parse_style(cell.get("style"))
            is_edge = cell.get("edge") == "1"

            if is_edge:
                text_bg = (
                    style.get("labelBackgroundColor") or self.parent_backdrop(cell)
                ).upper()
            else:
                text_bg = self.backdrop(cell)

            base = (style.get("fontColor") or DEFAULT_FONT_COLOR).upper()
            for text, color in label_runs(cell.get("value"), base):
                excerpt = text if len(text) <= 24 else text[:21] + "…"
                self.check(color, text_bg, TEXT_AA, f'{cell_id} "{excerpt}"')

            # Only arrows are held to the 3.0 non-text bar. Card and band
            # outlines are decorative: every card is already told apart by its
            # fill plus a high-contrast label, and the bands are labelled by
            # their own chip — nothing in the diagram is understood through an
            # outline alone.
            stroke = style.get("strokeColor")
            if is_edge and stroke and stroke.lower() != "none":
                self.check(
                    stroke.upper(),
                    self.parent_backdrop(cell),
                    STROKE_AA,
                    f"{cell_id} arrow",
                )
        return self.failures


def audit(path: Path) -> bool:
    failures = Audit(path).run()
    if failures:
        print(f"✗ {path.name}: {len(failures)} contrast failure(s)", file=sys.stderr)
        for line in failures:
            print(f"    {line}", file=sys.stderr)
        return False
    print(f"✓ {path.name}: WCAG AA (text {TEXT_AA}, stroke {STROKE_AA})")
    return True


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)

    gen = sub.add_parser("generate", help="dark .drawio -> light .drawio")
    gen.add_argument("source", type=Path)
    gen.add_argument("target", type=Path)

    aud = sub.add_parser("audit", help="check one .drawio for WCAG AA")
    aud.add_argument("file", type=Path)

    args = parser.parse_args()

    if args.command == "audit":
        return 0 if audit(args.file) else 1

    mapping = load_mapping()
    light = replace_colors(args.source.read_text(), mapping)
    args.target.write_text(light)
    print(f"generated {args.target} from {args.source} ({len(mapping)} tokens)")

    ok = audit(args.source) & audit(args.target)
    if not ok:
        args.target.unlink(missing_ok=True)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
