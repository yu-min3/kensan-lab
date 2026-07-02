"""Prompt evaluator for assessing AI context quality.

Evaluates conversation quality for each AI context over a period,
gathering quantitative metrics and qualitative LLM analysis.
Follows the same provider branching pattern as ProfileSummarizer/FactExtractor.
"""

from __future__ import annotations

import logging
import random
from datetime import date
from typing import Any
from uuid import UUID

from kensan_ai.config import get_settings
from kensan_ai.db.connection import get_connection
from kensan_ai.db.queries import prompt_evaluations as eval_queries
from kensan_ai.lib.ai_provider import LLMClient
from kensan_ai.lib.llm_utils import extract_json_from_response

logger = logging.getLogger(__name__)

EVALUATION_PROMPT = """あなたはAIシステムプロンプトの品質評価者です。
以下のシステムプロンプトと、それを使った会話の定量メトリクス・サンプルを分析してください。

## システムプロンプト
```
{system_prompt}
```

## 定量メトリクス
- 会話数: {interaction_count}
- 評価数: {rated_count}
- 平均評価: {avg_rating}
- ツール成功率: {tool_success_rate}
- 平均ターン数: {avg_turns}
- 平均トークン数: {avg_tokens}

## 会話サンプル
{samples}

## 評価指示
上記を分析し、以下の形式でJSON出力してください。
```json
{{
  "strengths": ["強み1", "強み2", ...],
  "weaknesses": ["弱み1", "弱み2", ...],
  "improvement_suggestions": ["改善提案1", "改善提案2", ...]
}}
```

- strengths: プロンプトの良い点（3項目程度）
- weaknesses: 改善が必要な点（3項目程度、なければ空配列）
- improvement_suggestions: 具体的な改善提案（3項目程度、なければ空配列）
- JSON以外の出力は不要です
"""


class PromptEvaluator:
    """Evaluates AI context quality using quantitative metrics and LLM analysis."""

    def __init__(self):
        self.llm = LLMClient()

    async def evaluate_context(
        self,
        context_id: UUID,
        context_name: str,
        situation: str,
        system_prompt: str,
        period_start: date,
        period_end: date,
        persona_prompt: str | None = None,
        user_id: UUID | None = None,
        force: bool = False,
    ) -> dict[str, Any]:
        """Evaluate an AI context for a given period.

        Returns:
            {
                "evaluation_id": UUID | None,
                "needs_improvement": bool,
                "metrics": {...},
                "qualitative": {"strengths": [], "weaknesses": [], "improvement_suggestions": []},
                "skipped_reason": str | None,
            }
        """
        # 1. Gather quantitative metrics
        metrics = await self._gather_metrics(context_id, period_start, period_end)

        # Skip if too few interactions (unless forced)
        if not force and metrics["interaction_count"] < 5:
            logger.info(
                "Skipping evaluation for %s: only %d interactions",
                context_name,
                metrics["interaction_count"],
            )
            return {
                "evaluation_id": None,
                "needs_improvement": False,
                "metrics": metrics,
                "qualitative": {"strengths": [], "weaknesses": [], "improvement_suggestions": []},
                "skipped_reason": "too_few_interactions",
            }

        # 2. Sample conversations
        samples = await self._sample_conversations(context_id, period_start, period_end)

        # 3. LLM qualitative analysis (include persona if available)
        eval_prompt = system_prompt
        if persona_prompt:
            eval_prompt = persona_prompt + "\n\n" + system_prompt
        qualitative = await self._analyze_quality(eval_prompt, metrics, samples)

        # 4. Delete existing evaluation if force mode (avoids unique constraint conflict)
        if force:
            async with get_connection() as conn:
                await conn.execute(
                    "DELETE FROM prompt_evaluations "
                    "WHERE context_id = $1 AND period_start = $2 AND user_id = $3",
                    context_id,
                    period_start,
                    user_id,
                )

        # 5. Save evaluation
        evaluation_id = await eval_queries.create_evaluation(
            context_id=context_id,
            period_start=period_start,
            period_end=period_end,
            interaction_count=metrics["interaction_count"],
            avg_rating=metrics["avg_rating"],
            rated_count=metrics["rated_count"],
            tool_success_rate=metrics["tool_success_rate"],
            avg_turns=metrics["avg_turns"],
            avg_tokens=metrics["avg_tokens"],
            strengths=qualitative["strengths"],
            weaknesses=qualitative["weaknesses"],
            improvement_suggestions=qualitative["improvement_suggestions"],
            sample_analysis={"sample_count": len(samples)},
            user_id=user_id,
        )

        if evaluation_id is None:
            logger.info("Evaluation already exists for %s period_start=%s", context_name, period_start)
            return {
                "evaluation_id": None,
                "needs_improvement": False,
                "metrics": metrics,
                "qualitative": qualitative,
                "skipped_reason": "already_evaluated",
            }

        # 5. Determine if improvement is needed
        needs_improvement = self._needs_improvement(metrics, qualitative)

        return {
            "evaluation_id": evaluation_id,
            "needs_improvement": needs_improvement,
            "metrics": metrics,
            "qualitative": qualitative,
            "skipped_reason": None,
        }

    async def _gather_metrics(
        self,
        context_id: UUID,
        period_start: date,
        period_end: date,
    ) -> dict[str, Any]:
        """Gather quantitative metrics from ai_interactions."""
        async with get_connection() as conn:
            row = await conn.fetchrow(
                """
                SELECT
                    COUNT(*) AS interaction_count,
                    AVG(rating) FILTER (WHERE rating IS NOT NULL) AS avg_rating,
                    COUNT(*) FILTER (WHERE rating IS NOT NULL) AS rated_count,
                    AVG(COALESCE(tokens_input, 0) + COALESCE(tokens_output, 0)) AS avg_tokens
                FROM ai_interactions
                WHERE context_id = $1
                  AND created_at >= $2::date
                  AND created_at < ($3::date + INTERVAL '1 day')
                """,
                context_id,
                period_start,
                period_end,
            )

            # Calculate avg_turns from conversations
            turns_row = await conn.fetchrow(
                """
                SELECT AVG(turn_count) AS avg_turns
                FROM (
                    SELECT conversation_id, COUNT(*) AS turn_count
                    FROM ai_interactions
                    WHERE context_id = $1
                      AND created_at >= $2::date
                      AND created_at < ($3::date + INTERVAL '1 day')
                      AND conversation_id IS NOT NULL
                    GROUP BY conversation_id
                ) sub
                """,
                context_id,
                period_start,
                period_end,
            )

        return {
            "interaction_count": row["interaction_count"],
            "avg_rating": float(row["avg_rating"]) if row["avg_rating"] is not None else None,
            "rated_count": row["rated_count"],
            "tool_success_rate": None,  # Not tracked per-interaction currently
            "avg_turns": float(turns_row["avg_turns"]) if turns_row["avg_turns"] is not None else None,
            "avg_tokens": float(row["avg_tokens"]) if row["avg_tokens"] is not None else None,
        }

    async def _sample_conversations(
        self,
        context_id: UUID,
        period_start: date,
        period_end: date,
        max_samples: int = 15,
    ) -> list[dict[str, Any]]:
        """Sample conversations: low-rated priority + high-rated + random."""
        async with get_connection() as conn:
            # Low-rated (priority)
            low_rated = await conn.fetch(
                """
                SELECT user_input, ai_output, rating, conversation_id
                FROM ai_interactions
                WHERE context_id = $1
                  AND created_at >= $2::date
                  AND created_at < ($3::date + INTERVAL '1 day')
                  AND rating IS NOT NULL AND rating <= 3
                ORDER BY rating ASC, created_at DESC
                LIMIT 5
                """,
                context_id,
                period_start,
                period_end,
            )

            # High-rated
            high_rated = await conn.fetch(
                """
                SELECT user_input, ai_output, rating, conversation_id
                FROM ai_interactions
                WHERE context_id = $1
                  AND created_at >= $2::date
                  AND created_at < ($3::date + INTERVAL '1 day')
                  AND rating IS NOT NULL AND rating >= 4
                ORDER BY rating DESC, created_at DESC
                LIMIT 5
                """,
                context_id,
                period_start,
                period_end,
            )

            # Random sample
            random_sample = await conn.fetch(
                """
                SELECT user_input, ai_output, rating, conversation_id
                FROM ai_interactions
                WHERE context_id = $1
                  AND created_at >= $2::date
                  AND created_at < ($3::date + INTERVAL '1 day')
                ORDER BY RANDOM()
                LIMIT 5
                """,
                context_id,
                period_start,
                period_end,
            )

        # Deduplicate by conversation_id
        seen_convs: set[str] = set()
        samples = []
        for row in list(low_rated) + list(high_rated) + list(random_sample):
            conv_id = str(row["conversation_id"]) if row["conversation_id"] else None
            if conv_id and conv_id in seen_convs:
                continue
            if conv_id:
                seen_convs.add(conv_id)
            samples.append({
                "user_input": (row["user_input"] or "")[:300],
                "ai_output": (row["ai_output"] or "")[:300],
                "rating": row["rating"],
            })
            if len(samples) >= max_samples:
                break

        return samples

    async def _analyze_quality(
        self,
        system_prompt: str,
        metrics: dict[str, Any],
        samples: list[dict[str, Any]],
    ) -> dict[str, Any]:
        """Use LLM to analyze prompt quality."""
        samples_text = ""
        for i, s in enumerate(samples, 1):
            rating_str = f" (評価: {s['rating']})" if s["rating"] is not None else ""
            samples_text += f"\n### サンプル {i}{rating_str}\nユーザー: {s['user_input']}\nAI: {s['ai_output']}\n"

        prompt = EVALUATION_PROMPT.format(
            system_prompt=system_prompt[:3000],
            interaction_count=metrics["interaction_count"],
            rated_count=metrics["rated_count"],
            avg_rating=f"{metrics['avg_rating']:.2f}" if metrics["avg_rating"] is not None else "N/A",
            tool_success_rate=f"{metrics['tool_success_rate']:.2f}" if metrics["tool_success_rate"] is not None else "N/A",
            avg_turns=f"{metrics['avg_turns']:.1f}" if metrics["avg_turns"] is not None else "N/A",
            avg_tokens=f"{metrics['avg_tokens']:.0f}" if metrics["avg_tokens"] is not None else "N/A",
            samples=samples_text,
        )

        try:
            content = await self.llm.generate(prompt, max_tokens=2048)
            data = extract_json_from_response(content or "{}")
            return {
                "strengths": data.get("strengths", [])[:5],
                "weaknesses": data.get("weaknesses", [])[:5],
                "improvement_suggestions": data.get("improvement_suggestions", [])[:5],
            }

        except Exception as e:
            logger.error("Quality analysis failed: %s", e)
            return {"strengths": [], "weaknesses": [], "improvement_suggestions": []}

    def _needs_improvement(
        self,
        metrics: dict[str, Any],
        qualitative: dict[str, Any],
    ) -> bool:
        """Determine if the prompt needs improvement."""
        # Low average rating
        if metrics["avg_rating"] is not None and metrics["avg_rating"] < 3.5:
            return True

        # Multiple weaknesses identified
        if len(qualitative.get("weaknesses", [])) >= 2:
            return True

        return False
