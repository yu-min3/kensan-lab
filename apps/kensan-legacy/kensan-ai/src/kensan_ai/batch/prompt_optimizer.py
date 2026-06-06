"""Prompt optimizer for generating improved system prompts.

Takes the current prompt, evaluation weaknesses, and improvement suggestions,
then uses LLM to generate an improved version while preserving template variables
and structural constraints.
"""

from __future__ import annotations

import logging
import re
from typing import Any
from uuid import UUID

from kensan_ai.config import get_settings
from kensan_ai.db.connection import get_connection
from kensan_ai.lib.ai_provider import LLMClient

logger = logging.getLogger(__name__)

OPTIMIZATION_PROMPT = """あなたはAIシステムプロンプトの最適化専門家です。
以下の現行プロンプトの弱点と改善提案を踏まえ、改善版を生成してください。

{persona_section}## 現行プロンプト
```
{current_prompt}
```

## 評価結果
### 弱点
{weaknesses}

### 改善提案
{suggestions}

## 制約（必ず守ること）
1. テンプレート変数を保持する: {template_vars}
2. プロンプトの長さは現行の±20%以内にする（現行: {current_length}文字）
3. プロンプトの基本構造（セクション分け等）は維持する
4. 日本語で出力する

## 出力形式
以下のデリミタ形式で出力してください。各セクションをデリミタで囲んでください。

===CHANGELOG_START===
主な変更点の要約（1-2文）
===CHANGELOG_END===

===CHANGES_START===
変更箇所のリスト
===CHANGES_END===

===PROMPT_START===
改善されたシステムプロンプト全文
===PROMPT_END===
"""


def _parse_delimited_output(content: str) -> dict[str, str]:
    """Parse delimiter-based LLM output into structured fields."""
    result: dict[str, str] = {}

    for key, start_tag, end_tag in [
        ("changelog", "===CHANGELOG_START===", "===CHANGELOG_END==="),
        ("changes_summary", "===CHANGES_START===", "===CHANGES_END==="),
        ("improved_prompt", "===PROMPT_START===", "===PROMPT_END==="),
    ]:
        s = content.find(start_tag)
        e = content.find(end_tag)
        if s != -1 and e != -1:
            result[key] = content[s + len(start_tag):e].strip()

    return result


class PromptOptimizer:
    """Generates improved system prompts using LLM analysis."""

    # Regex to find template variables like {user_memory}, {today_schedule}
    TEMPLATE_VAR_PATTERN = re.compile(r"\{(\w+)\}")

    def __init__(self):
        self.llm = LLMClient()

    async def generate_improved_prompt(
        self,
        current_prompt: str,
        weaknesses: list[str],
        suggestions: list[str],
        persona_prompt: str | None = None,
    ) -> dict[str, Any] | None:
        """Generate an improved version of the system prompt.

        Returns:
            {"improved_prompt": str, "changelog": str, "changes_summary": str} or None on failure
        """
        # Identify template variables in current prompt
        template_vars = list(set(self.TEMPLATE_VAR_PATTERN.findall(current_prompt)))
        template_vars_str = ", ".join(f"{{{v}}}" for v in template_vars) if template_vars else "なし"

        weaknesses_text = "\n".join(f"- {w}" for w in weaknesses) if weaknesses else "- 特になし"
        suggestions_text = "\n".join(f"- {s}" for s in suggestions) if suggestions else "- 特になし"

        persona_section = ""
        if persona_prompt:
            persona_section = (
                "## ペルソナプロンプト（変更不可・参照のみ）\n"
                "```\n"
                f"{persona_prompt}\n"
                "```\n"
                "注意: 上記ペルソナは全 situation に共通で適用される。"
                "最適化対象はペルソナ以降の部分のみ。\n\n"
            )

        prompt = OPTIMIZATION_PROMPT.format(
            current_prompt=current_prompt,
            persona_section=persona_section,
            weaknesses=weaknesses_text,
            suggestions=suggestions_text,
            template_vars=template_vars_str,
            current_length=len(current_prompt),
        )

        try:
            content = await self.llm.generate(prompt, max_tokens=16384)
            data = _parse_delimited_output(content or "")

            improved_prompt = data.get("improved_prompt", "")
            if not improved_prompt:
                logger.error("Empty improved prompt received")
                return None

            # Validate template variables are preserved
            improved_vars = set(self.TEMPLATE_VAR_PATTERN.findall(improved_prompt))
            missing_vars = set(template_vars) - improved_vars
            if missing_vars:
                logger.warning(
                    "Improved prompt missing template vars: %s. Skipping.",
                    missing_vars,
                )
                return None

            # Validate length constraint (±20%)
            length_ratio = len(improved_prompt) / len(current_prompt) if current_prompt else 1.0
            if length_ratio < 0.8 or length_ratio > 1.2:
                logger.warning(
                    "Improved prompt length ratio %.2f outside ±20%%. Accepting anyway.",
                    length_ratio,
                )

            return {
                "improved_prompt": improved_prompt,
                "changelog": data.get("changelog", "自動最適化による改善"),
                "changes_summary": data.get("changes_summary", ""),
            }

        except Exception as e:
            logger.error("Prompt optimization failed: %s", e)
            return None

    async def create_candidate_version(
        self,
        context_id: UUID,
        improved_prompt: str,
        changelog: str,
        eval_summary: dict | None = None,
    ) -> int | None:
        """Create a new candidate version in ai_context_versions.

        Does NOT update ai_contexts (the candidate is not yet adopted).
        Sets source='ai' and candidate_status='pending'.

        Returns the new version number, or None on failure.
        """
        async with get_connection() as conn:
            # Get source context to copy settings
            source = await conn.fetchrow(
                """
                SELECT allowed_tools, max_turns, temperature
                FROM ai_contexts
                WHERE id = $1
                """,
                context_id,
            )
            if source is None:
                logger.error("Context %s not found", context_id)
                return None

            # Determine next version number
            latest_version = await conn.fetchval(
                "SELECT COALESCE(MAX(version_number), 0) FROM ai_context_versions WHERE context_id = $1",
                context_id,
            )
            next_version = latest_version + 1

            # Insert candidate version with metadata
            import json
            eval_summary_json = json.dumps(eval_summary, ensure_ascii=False) if eval_summary else None

            await conn.execute(
                """
                INSERT INTO ai_context_versions (
                    context_id, version_number, system_prompt, allowed_tools,
                    max_turns, temperature, changelog,
                    source, eval_summary, candidate_status
                )
                VALUES ($1, $2, $3, $4, $5, $6, $7, 'ai', $8, 'pending')
                """,
                context_id,
                next_version,
                improved_prompt,
                list(source["allowed_tools"]),
                source["max_turns"],
                source["temperature"],
                f"AI最適化: {changelog}",
                eval_summary_json,
            )

            logger.info(
                "Created candidate version %d for context %s",
                next_version,
                context_id,
            )
            return next_version
