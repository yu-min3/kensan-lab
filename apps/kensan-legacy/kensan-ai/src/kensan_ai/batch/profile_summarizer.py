"""Profile summarizer for aggregating user facts into profile summaries."""

import logging
from datetime import datetime, timedelta
from typing import Any
from uuid import UUID
from zoneinfo import ZoneInfo

from kensan_ai.config import get_settings
from kensan_ai.db.connection import get_connection
from kensan_ai.lib.ai_provider import LLMClient

logger = logging.getLogger(__name__)

SUMMARIZATION_PROMPT = """以下のユーザーに関する事実情報を基に、簡潔なプロフィールサマリーを作成してください。

【既存のサマリー】
{existing_summary}

【新しい事実】
{new_facts}

【作成ルール】
1. 300文字以内で簡潔にまとめてください
2. ユーザーの特徴、好み、目標、スキルを把握できる内容にしてください
3. 具体的で実用的な情報を優先してください
4. 矛盾する情報がある場合は新しい情報を優先してください
5. 日本語で出力してください

【出力形式】
プロフィールサマリーのテキストのみを出力してください。説明や前置きは不要です。
"""


class ProfileSummarizer:
    """Summarizes user facts into profile summaries using LLM API."""

    def __init__(self):
        """Initialize the profile summarizer with the configured AI provider."""
        self.llm = LLMClient()

    async def get_users_with_new_facts(
        self,
        updated_since: datetime,
    ) -> list[UUID]:
        """Get users who have new facts since the given time.

        Args:
            updated_since: Only consider facts created after this time

        Returns:
            List of user IDs with new facts
        """
        async with get_connection() as conn:
            rows = await conn.fetch(
                """
                SELECT DISTINCT user_id
                FROM user_facts
                WHERE created_at > $1
                """,
                updated_since,
            )
            return [row["user_id"] for row in rows]

    async def get_user_facts(
        self,
        user_id: UUID,
        limit: int = 50,
    ) -> list[dict[str, Any]]:
        """Get all active facts for a user.

        Args:
            user_id: The user's ID
            limit: Maximum number of facts to retrieve

        Returns:
            List of fact dictionaries
        """
        async with get_connection() as conn:
            rows = await conn.fetch(
                """
                SELECT fact_type, content, confidence, created_at
                FROM user_facts
                WHERE user_id = $1
                  AND (expires_at IS NULL OR expires_at > NOW())
                ORDER BY confidence DESC, created_at DESC
                LIMIT $2
                """,
                user_id,
                limit,
            )
            return [
                {
                    "type": row["fact_type"],
                    "content": row["content"],
                    "confidence": row["confidence"],
                }
                for row in rows
            ]

    async def get_existing_summary(self, user_id: UUID) -> str | None:
        """Get the existing profile summary for a user.

        Args:
            user_id: The user's ID

        Returns:
            The existing profile summary, or None
        """
        async with get_connection() as conn:
            row = await conn.fetchrow(
                """
                SELECT profile_summary
                FROM user_memory
                WHERE user_id = $1
                """,
                user_id,
            )
            return row["profile_summary"] if row else None

    async def generate_summary(
        self,
        existing_summary: str | None,
        facts: list[dict[str, Any]],
    ) -> str:
        """Generate a profile summary from facts.

        Args:
            existing_summary: The existing profile summary (may be None)
            facts: List of fact dictionaries

        Returns:
            The generated profile summary
        """
        # Format facts for the prompt
        facts_text = "\n".join(
            f"- [{f['type']}] {f['content']} (確信度: {f['confidence']:.1f})"
            for f in facts
        )

        if not facts_text:
            facts_text = "（新しい事実なし）"

        prompt = SUMMARIZATION_PROMPT.format(
            existing_summary=existing_summary or "（なし）",
            new_facts=facts_text,
        )

        result = await self.llm.generate(prompt, max_tokens=500)
        return result.strip()

    async def update_user_memory(
        self,
        user_id: UUID,
        profile_summary: str,
        facts: list[dict[str, Any]],
    ) -> bool:
        """Update or create user_memory record.

        Args:
            user_id: The user's ID
            profile_summary: The new profile summary
            facts: The facts used for summarization

        Returns:
            True if successful
        """
        # Extract strengths and growth areas from facts
        strengths = [
            f["content"] for f in facts
            if f["type"] == "skill" and f["confidence"] >= 0.7
        ][:5]  # Top 5 strengths

        growth_areas = [
            f["content"] for f in facts
            if f["type"] in ("goal", "constraint")
        ][:5]  # Top 5 growth areas

        async with get_connection() as conn:
            # Upsert user_memory
            await conn.execute(
                """
                INSERT INTO user_memory (user_id, profile_summary, strengths, growth_areas, last_updated)
                VALUES ($1, $2, $3, $4, NOW())
                ON CONFLICT (user_id) DO UPDATE SET
                    profile_summary = EXCLUDED.profile_summary,
                    strengths = EXCLUDED.strengths,
                    growth_areas = EXCLUDED.growth_areas,
                    last_updated = NOW()
                """,
                user_id,
                profile_summary,
                strengths,
                growth_areas,
            )
            return True

    async def summarize_user(self, user_id: UUID) -> bool:
        """Generate and save a profile summary for a user.

        Args:
            user_id: The user's ID

        Returns:
            True if successful
        """
        try:
            # Get existing summary and facts
            existing_summary = await self.get_existing_summary(user_id)
            facts = await self.get_user_facts(user_id)

            if not facts:
                logger.info(f"No facts found for user {user_id}, skipping")
                return False

            # Generate new summary
            new_summary = await self.generate_summary(existing_summary, facts)

            if not new_summary:
                logger.warning(f"Empty summary generated for user {user_id}")
                return False

            # Update user memory
            await self.update_user_memory(user_id, new_summary, facts)
            logger.info(f"Updated profile summary for user {user_id}")
            return True

        except Exception as e:
            logger.error(f"Failed to summarize user {user_id}: {e}")
            return False

    async def run_batch(
        self,
        updated_since: datetime | None = None,
        days: int = 1,
    ) -> int:
        """Run batch summarization for users with new facts.

        Args:
            updated_since: Only process users with facts after this time
            days: Alternative to updated_since - process facts from last N days

        Returns:
            Number of users processed
        """
        if updated_since is None:
            updated_since = datetime.now(ZoneInfo("UTC")) - timedelta(days=days)

        logger.info(f"Starting profile summarization batch (since {updated_since})")

        # Get users with new facts
        users = await self.get_users_with_new_facts(updated_since)
        logger.info(f"Found {len(users)} users with new facts")

        # Process each user
        processed = 0
        for user_id in users:
            if await self.summarize_user(user_id):
                processed += 1

        logger.info(f"Batch complete: processed {processed}/{len(users)} users")
        return processed
