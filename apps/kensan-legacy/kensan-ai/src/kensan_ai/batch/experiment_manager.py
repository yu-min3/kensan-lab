"""Experiment manager for orchestrating prompt optimization batch.

Evaluates all active default contexts, generates improved prompts for
those needing improvement, and creates candidate versions for user review.
"""

from __future__ import annotations

import logging
from datetime import date, timedelta

from kensan_ai.lib.timezone_utils import local_today
from typing import Any

from kensan_ai.batch.prompt_evaluator import PromptEvaluator
from kensan_ai.batch.prompt_optimizer import PromptOptimizer
from kensan_ai.db.connection import get_connection

logger = logging.getLogger(__name__)


def _last_week_range() -> tuple[date, date]:
    """Return (7 days ago, today) as the evaluation period."""
    today = local_today()
    period_start = today - timedelta(days=7)
    period_end = today
    return period_start, period_end


async def run_prompt_optimization_batch() -> dict[str, Any]:
    """Run the full prompt optimization batch.

    1. Get all active default contexts (excluding persona)
    2. Evaluate each context
    3. For contexts needing improvement, generate optimized prompt
    4. Create candidate versions for user review

    Returns:
        {"contexts_evaluated": N, "candidates_created": N, "errors": [...]}
    """
    period_start, period_end = _last_week_range()
    logger.info(
        "Starting prompt optimization batch for %s ~ %s",
        period_start,
        period_end,
    )

    # 1. Get all users who have per-user contexts
    async with get_connection() as conn:
        user_rows = await conn.fetch(
            "SELECT DISTINCT user_id FROM ai_contexts WHERE user_id IS NOT NULL AND is_active = true"
        )

    if not user_rows:
        logger.info("No users with per-user contexts found")
        return {"contexts_evaluated": 0, "candidates_created": 0, "errors": []}

    evaluator = PromptEvaluator()
    optimizer = PromptOptimizer()

    contexts_evaluated = 0
    candidates_created = 0
    errors: list[str] = []

    for user_row in user_rows:
        user_id = user_row["user_id"]
        try:
            stats = await _process_user_contexts(
                user_id=user_id,
                period_start=period_start,
                period_end=period_end,
                evaluator=evaluator,
                optimizer=optimizer,
            )
            contexts_evaluated += stats["contexts_evaluated"]
            candidates_created += stats["candidates_created"]
            errors.extend(stats["errors"])
        except Exception:
            logger.exception("Error processing user %s", user_id)
            errors.append(f"user_{user_id}: unexpected_error")

    summary = {
        "period_start": period_start.isoformat(),
        "period_end": period_end.isoformat(),
        "users_processed": len(user_rows),
        "contexts_evaluated": contexts_evaluated,
        "candidates_created": candidates_created,
        "errors": errors,
    }
    logger.info("Prompt optimization batch complete: %s", summary)
    return summary


async def _process_user_contexts(
    user_id: Any,
    period_start: date,
    period_end: date,
    evaluator: PromptEvaluator,
    optimizer: PromptOptimizer,
    force: bool = False,
) -> dict[str, Any]:
    """Process all active default contexts for a single user.

    Returns:
        {"contexts_evaluated": N, "candidates_created": N, "errors": [...]}
    """
    result: dict[str, Any] = {"contexts_evaluated": 0, "candidates_created": 0, "errors": [], "optimized_context_ids": []}

    async with get_connection() as conn:
        contexts = await conn.fetch(
            """
            SELECT id, name, situation, system_prompt
            FROM ai_contexts
            WHERE user_id = $1
              AND is_active = true
              AND is_default = true
              AND situation != 'persona'
            ORDER BY situation
            """,
            user_id,
        )

        # Fetch this user's persona prompt
        persona_row = await conn.fetchrow(
            "SELECT system_prompt FROM ai_contexts "
            "WHERE user_id = $1 AND situation = 'persona' AND is_active = true LIMIT 1",
            user_id,
        )

    if not contexts:
        return result

    persona_prompt = persona_row["system_prompt"] if persona_row else None
    logger.info("Processing %d contexts for user %s", len(contexts), user_id)

    for ctx in contexts:
        ctx_name = ctx["name"]
        try:
            eval_result = await evaluator.evaluate_context(
                context_id=ctx["id"],
                context_name=ctx_name,
                situation=ctx["situation"],
                system_prompt=ctx["system_prompt"],
                period_start=period_start,
                period_end=period_end,
                persona_prompt=persona_prompt,
                user_id=user_id,
                force=force,
            )
            result["contexts_evaluated"] += 1

            if not force and eval_result["skipped_reason"]:
                logger.info("Skipped %s: %s", ctx_name, eval_result["skipped_reason"])
                continue

            if not force and not eval_result["needs_improvement"]:
                logger.info("No improvement needed for %s", ctx_name)
                continue

            optimization = await optimizer.generate_improved_prompt(
                current_prompt=ctx["system_prompt"],
                weaknesses=eval_result["qualitative"]["weaknesses"],
                suggestions=eval_result["qualitative"]["improvement_suggestions"],
                persona_prompt=persona_prompt,
            )

            if optimization is None:
                logger.warning("Failed to generate improved prompt for %s", ctx_name)
                result["errors"].append(f"{ctx_name}: optimization_failed")
                continue

            # Build eval_summary from evaluation result
            eval_summary = {
                "interaction_count": eval_result["metrics"]["interaction_count"],
                "avg_rating": eval_result["metrics"]["avg_rating"],
                "strengths": eval_result["qualitative"]["strengths"],
                "weaknesses": eval_result["qualitative"]["weaknesses"],
            }

            variant_version = await optimizer.create_candidate_version(
                context_id=ctx["id"],
                improved_prompt=optimization["improved_prompt"],
                changelog=optimization["changelog"],
                eval_summary=eval_summary,
            )

            if variant_version is None:
                logger.warning("Failed to create candidate version for %s", ctx_name)
                result["errors"].append(f"{ctx_name}: variant_creation_failed")
                continue

            result["candidates_created"] += 1
            result["optimized_context_ids"].append(str(ctx["id"]))
            logger.info(
                "Created candidate version %d for %s (eval_id=%s, user=%s)",
                variant_version,
                ctx_name,
                eval_result["evaluation_id"],
                user_id,
            )

        except Exception:
            logger.exception("Error processing context %s", ctx_name)
            result["errors"].append(f"{ctx_name}: unexpected_error")

    return result
