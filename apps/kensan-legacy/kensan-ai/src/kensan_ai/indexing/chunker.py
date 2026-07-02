"""Content chunking for note indexing.

Splits note content into chunks suitable for embedding and search.
Strategies vary by content_type:
- markdown: split by headings (h1-h3), sub-split long sections by paragraph
- code: fixed-length with overlap
- drawio: extract labels into a single chunk
- image, pdf: skip (return empty list)
"""

import re
import xml.etree.ElementTree as ET
from dataclasses import dataclass

# Rough token estimation: ~1.5 chars per token for Japanese, ~4 for English.
# We use a blended estimate of ~2.5 chars per token for mixed content.
_CHARS_PER_TOKEN = 2.5


@dataclass
class Chunk:
    index: int
    text: str
    token_count: int


def estimate_tokens(text: str) -> int:
    """Estimate token count for mixed Japanese/English text."""
    if not text:
        return 0
    return max(1, int(len(text) / _CHARS_PER_TOKEN))


def chunk_content(text: str, content_type: str) -> list[Chunk]:
    """Route to the appropriate chunker based on content_type.

    Args:
        text: The raw content text.
        content_type: One of 'markdown', 'code', 'drawio', 'image', 'pdf'.

    Returns:
        List of Chunk objects. Empty for unsupported types.
    """
    if not text or not text.strip():
        return []

    if content_type == "markdown":
        return chunk_markdown(text)
    elif content_type == "code":
        return chunk_code(text)
    elif content_type == "drawio":
        return chunk_drawio(text)
    else:
        # image, pdf, etc. → skip
        return []


def chunk_markdown(text: str, max_tokens: int = 500) -> list[Chunk]:
    """Split markdown by headings (h1-h3), sub-split long sections by paragraph.

    Each heading starts a new section. If a section exceeds max_tokens,
    it is further split at paragraph boundaries (blank lines).
    """
    # Split by headings (keep the heading with its section)
    heading_pattern = re.compile(r"^(#{1,3}\s)", re.MULTILINE)
    parts = heading_pattern.split(text)

    # Reassemble: parts[0] is text before first heading,
    # then pairs of (heading_marker, section_text)
    sections: list[str] = []
    if parts[0].strip():
        sections.append(parts[0].strip())

    i = 1
    while i < len(parts) - 1:
        heading_marker = parts[i]
        section_text = parts[i + 1] if i + 1 < len(parts) else ""
        sections.append((heading_marker + section_text).strip())
        i += 2

    # Sub-split long sections by paragraph
    chunks: list[Chunk] = []
    idx = 0
    for section in sections:
        if not section:
            continue
        tokens = estimate_tokens(section)
        if tokens <= max_tokens:
            chunks.append(Chunk(index=idx, text=section, token_count=tokens))
            idx += 1
        else:
            # Split by blank lines (paragraphs)
            paragraphs = re.split(r"\n\s*\n", section)
            buffer = ""
            for para in paragraphs:
                para = para.strip()
                if not para:
                    continue
                candidate = (buffer + "\n\n" + para).strip() if buffer else para
                if estimate_tokens(candidate) > max_tokens and buffer:
                    # Flush buffer
                    chunks.append(Chunk(index=idx, text=buffer, token_count=estimate_tokens(buffer)))
                    idx += 1
                    buffer = para
                else:
                    buffer = candidate
            if buffer:
                chunks.append(Chunk(index=idx, text=buffer, token_count=estimate_tokens(buffer)))
                idx += 1

    return chunks


def chunk_code(text: str, max_tokens: int = 500, overlap_tokens: int = 50) -> list[Chunk]:
    """Split code into fixed-length chunks with overlap.

    Args:
        text: Source code text.
        max_tokens: Maximum tokens per chunk.
        overlap_tokens: Token overlap between consecutive chunks.
    """
    max_chars = int(max_tokens * _CHARS_PER_TOKEN)
    overlap_chars = int(overlap_tokens * _CHARS_PER_TOKEN)

    chunks: list[Chunk] = []
    idx = 0
    start = 0

    while start < len(text):
        end = start + max_chars
        chunk_text = text[start:end].strip()
        if chunk_text:
            chunks.append(Chunk(index=idx, text=chunk_text, token_count=estimate_tokens(chunk_text)))
            idx += 1
        start = end - overlap_chars if end < len(text) else len(text)

    return chunks


def chunk_drawio(xml_text: str) -> list[Chunk]:
    """Extract text labels from drawio XML and produce a single chunk.

    Drawio files are XML with <mxCell> elements containing value attributes.
    We extract all non-empty value attributes and join them.
    """
    try:
        root = ET.fromstring(xml_text)
    except ET.ParseError:
        # If XML parsing fails, treat as plain text single chunk
        tokens = estimate_tokens(xml_text)
        return [Chunk(index=0, text=xml_text[:2000], token_count=min(tokens, 800))] if xml_text.strip() else []

    labels: list[str] = []
    for elem in root.iter():
        value = elem.get("value", "").strip()
        if value:
            # Remove HTML tags that drawio sometimes uses in labels
            clean = re.sub(r"<[^>]+>", " ", value).strip()
            if clean:
                labels.append(clean)

    if not labels:
        return []

    combined = "\n".join(labels)
    return [Chunk(index=0, text=combined, token_count=estimate_tokens(combined))]
