"""Fact extractor for automatic extraction of user facts from conversations."""

import json
import logging
from dataclasses import dataclass
from typing import Any
from uuid import UUID

from kensan_ai.config import get_settings
from kensan_ai.db.connection import get_connection
from kensan_ai.lib.ai_provider import LLMClient
from kensan_ai.lib.llm_utils import extract_json_from_response

logger = logging.getLogger(__name__)

EXTRACTION_PROMPT = """以下の会話からユーザーに関する事実を抽出してください。

【重要なルール】
- ユーザーが明示的に述べた事実のみを抽出してください
- 過度な推論や憶測は避けてください
- AIの発言からではなく、ユーザーの発言から事実を抽出してください
- 同じ内容を重複して抽出しないでください

【抽出するファクトの種類】
- preference: 好み、嗜好（例: 「朝型である」「カフェで作業するのが好き」）
- habit: 習慣、ルーティン（例: 「毎朝7時に起きる」「週末は読書する」）
- skill: スキル、能力（例: 「Pythonが得意」「英語が話せる」）
- goal: 目標、やりたいこと（例: 「来月までにアプリをリリースしたい」）
- constraint: 制約、条件（例: 「平日は19時以降しか作業できない」）

【出力形式】
JSON配列で出力してください。抽出する事実がない場合は空の配列[]を返してください。
```json
[
  {
    "type": "preference|habit|skill|goal|constraint",
    "content": "抽出した事実の内容（簡潔に）",
    "confidence": 0.0〜1.0（確信度）
  }
]
```

【会話】
ユーザー: {user_input}

AI: {ai_output}
"""


@dataclass
class ExtractedFact:
    """A fact extracted from a conversation."""

    type: str
    content: str
    confidence: float


class FactExtractor:
    """Extracts facts from AI conversations using LLM API."""

    # Valid fact types
    VALID_TYPES = {"preference", "habit", "skill", "goal", "constraint"}

    def __init__(self):
        """Initialize the fact extractor with the configured AI provider."""
        self.llm = LLMClient()

    async def extract_facts(
        self,
        user_input: str,
        ai_output: str,
    ) -> list[ExtractedFact]:
        """Extract facts from a conversation exchange.

        Args:
            user_input: The user's input message
            ai_output: The AI's response

        Returns:
            List of extracted facts
        """
        prompt = EXTRACTION_PROMPT.format(
            user_input=user_input,
            ai_output=ai_output,
        )

        try:
            content = await self.llm.generate(prompt, max_tokens=1024)
            facts_data = extract_json_from_response(content or "[]")

            # Validate and convert to ExtractedFact objects
            facts = []
            for item in facts_data:
                if not isinstance(item, dict):
                    continue

                fact_type = item.get("type", "")
                content_text = item.get("content", "")
                confidence = item.get("confidence", 0.8)

                # Validate
                if fact_type not in self.VALID_TYPES:
                    continue
                if not content_text or len(content_text) < 3:
                    continue
                if not 0.0 <= confidence <= 1.0:
                    confidence = 0.8

                facts.append(ExtractedFact(
                    type=fact_type,
                    content=content_text,
                    confidence=float(confidence),
                ))

            return facts

        except Exception as e:
            logger.error(f"Fact extraction failed: {e}")
            return []

    async def extract_and_save(
        self,
        user_id: UUID,
        user_input: str,
        ai_output: str,
        interaction_id: UUID | None = None,
    ) -> int:
        """Extract facts from a conversation and save them to the database.

        Args:
            user_id: The user's ID
            user_input: The user's input message
            ai_output: The AI's response
            interaction_id: Optional ID of the source interaction

        Returns:
            Number of facts saved
        """
        # Extract facts
        facts = await self.extract_facts(user_input, ai_output)

        if not facts:
            return 0

        # Save facts to database
        saved_count = 0
        async with get_connection() as conn:
            for fact in facts:
                try:
                    # Check for duplicate facts
                    existing = await conn.fetchrow(
                        """
                        SELECT id FROM user_facts
                        WHERE user_id = $1 AND fact_type = $2 AND content = $3
                        """,
                        user_id,
                        fact.type,
                        fact.content,
                    )

                    if existing:
                        # Update confidence if higher
                        await conn.execute(
                            """
                            UPDATE user_facts
                            SET confidence = GREATEST(confidence, $1),
                                source_interaction_id = COALESCE(source_interaction_id, $2)
                            WHERE id = $3
                            """,
                            fact.confidence,
                            interaction_id,
                            existing["id"],
                        )
                    else:
                        # Insert new fact
                        await conn.execute(
                            """
                            INSERT INTO user_facts (
                                user_id, fact_type, content, source, confidence, source_interaction_id
                            )
                            VALUES ($1, $2, $3, 'ai_extraction', $4, $5)
                            """,
                            user_id,
                            fact.type,
                            fact.content,
                            fact.confidence,
                            interaction_id,
                        )
                        saved_count += 1

                except Exception as e:
                    logger.error(f"Failed to save fact: {e}")

        logger.info(f"Extracted and saved {saved_count} facts for user {user_id}")
        return saved_count


# Global instance for reuse
_extractor: FactExtractor | None = None


def get_fact_extractor() -> FactExtractor:
    """Get or create the global FactExtractor instance."""
    global _extractor
    if _extractor is None:
        _extractor = FactExtractor()
    return _extractor
