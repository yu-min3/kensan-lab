"""Shared LLM response utilities."""

from __future__ import annotations

import json
import re


def _sanitize_json_strings(text: str) -> str:
    """Escape unescaped control characters inside JSON string values.

    LLMs sometimes produce JSON with literal newlines/tabs inside string
    values instead of proper ``\\n`` / ``\\t`` escapes, which causes
    ``json.loads`` to fail.  This helper walks through the text and escapes
    control characters that appear inside quoted strings.
    """
    result: list[str] = []
    in_string = False
    i = 0
    while i < len(text):
        ch = text[i]
        if ch == '"' and (i == 0 or text[i - 1] != "\\"):
            in_string = not in_string
            result.append(ch)
        elif in_string and ch == "\n":
            result.append("\\n")
        elif in_string and ch == "\r":
            result.append("\\r")
        elif in_string and ch == "\t":
            result.append("\\t")
        elif in_string and ord(ch) < 0x20:
            result.append(f"\\u{ord(ch):04x}")
        else:
            result.append(ch)
        i += 1
    return "".join(result)


def extract_json_from_response(content: str) -> dict | list:
    """Extract JSON from an LLM response, handling markdown code blocks.

    Supports responses wrapped in ```json ... ``` or bare ``` ... ``` blocks,
    as well as plain JSON.  Also sanitizes unescaped control characters that
    LLMs sometimes emit inside JSON string values.

    Args:
        content: Raw LLM response text.

    Returns:
        Parsed JSON as dict or list.

    Raises:
        json.JSONDecodeError: If the content cannot be parsed as JSON.
    """
    text = content.strip()

    if "```json" in text:
        start = text.find("```json") + 7
        end = text.find("```", start)
        text = text[start:end].strip()
    elif "```" in text:
        start = text.find("```") + 3
        end = text.find("```", start)
        text = text[start:end].strip()

    try:
        return json.loads(text)
    except json.JSONDecodeError:
        # Retry with sanitized control characters
        return json.loads(_sanitize_json_strings(text))
