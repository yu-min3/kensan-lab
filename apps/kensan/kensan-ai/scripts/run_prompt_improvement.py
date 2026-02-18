#!/usr/bin/env python3
"""Generate improved prompts based on agent evaluation results.

Reads evaluation output from run_agent_evaluation.py, fetches current prompts
via API, aggregates weaknesses per situation, then calls PromptOptimizer to
generate improved versions with before/after diff.

Usage:
    cd kensan-ai
    # First run evaluation:
    uv run python scripts/run_agent_evaluation.py

    # Then run improvement:
    uv run python scripts/run_prompt_improvement.py

    # Or point to a specific evaluation file:
    uv run python scripts/run_prompt_improvement.py --input /path/to/evaluation_results.json

    # Limit to a specific situation:
    uv run python scripts/run_prompt_improvement.py --situation chat
"""

from __future__ import annotations

import argparse
import asyncio
import difflib
import json
import logging
import os
import sys
from collections import defaultdict
from typing import Any

import httpx

# Add src to path so we can import kensan_ai modules
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from kensan_ai.batch.prompt_optimizer import PromptOptimizer

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(message)s",
)
logger = logging.getLogger(__name__)

BASE_URL = os.getenv("KENSAN_AI_URL", "http://kensan-ai-service:8089/api/v1")
AUTH_URL = os.getenv("KENSAN_AUTH_URL", "http://user-service:8081")
TEST_EMAIL = "test@kensan.dev"
TEST_PASSWORD = "password123"

DEFAULT_INPUT = os.path.join("/tmp", "evaluation_results.json")
DEFAULT_OUTPUT = os.path.join("/tmp", "improvement_results.json")


# =========================================================================
# Helpers
# =========================================================================


async def get_jwt_token() -> str:
    """Login and get JWT token."""
    async with httpx.AsyncClient() as client:
        resp = await client.post(
            f"{AUTH_URL}/api/v1/auth/login",
            json={"email": TEST_EMAIL, "password": TEST_PASSWORD},
        )
        resp.raise_for_status()
        return resp.json()["data"]["token"]


async def fetch_prompts(token: str, situation: str | None = None) -> list[dict]:
    """Fetch current prompts via API."""
    async with httpx.AsyncClient() as client:
        params = {}
        if situation:
            params["situation"] = situation
        resp = await client.get(
            f"{BASE_URL}/prompts",
            headers={"Authorization": f"Bearer {token}"},
            params=params,
        )
        resp.raise_for_status()
        return resp.json()


def load_evaluation_results(path: str) -> list[dict]:
    """Load evaluation results from JSON file."""
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def aggregate_by_situation(results: list[dict]) -> dict[str, dict[str, Any]]:
    """Aggregate evaluation results per situation.

    Returns:
        {situation: {weaknesses: [...], suggestions: [...], scores: {...}, scenarios: [...]}}
    """
    by_situation: dict[str, dict[str, Any]] = {}

    for r in results:
        if "error" in r:
            continue

        sit = r.get("situation", "unknown")
        if sit not in by_situation:
            by_situation[sit] = {
                "weaknesses": [],
                "suggestions": [],
                "scores": defaultdict(list),
                "scenarios": [],
            }

        entry = by_situation[sit]
        ev = r.get("evaluation", {})

        # Collect scores
        for dim, data in ev.get("scores", {}).items():
            if isinstance(data, dict) and isinstance(data.get("score"), (int, float)):
                entry["scores"][dim].append(data["score"])

        # Extract weaknesses from checklist_violations
        for v in ev.get("checklist_violations", []):
            if v and v not in entry["weaknesses"]:
                entry["weaknesses"].append(v)

        # Extract weaknesses from low-scoring dimensions
        for dim, data in ev.get("scores", {}).items():
            if isinstance(data, dict):
                score = data.get("score", 5)
                reason = data.get("reason", "")
                if score <= 2 and reason:
                    weakness = f"[{dim}] {reason}"
                    if weakness not in entry["weaknesses"]:
                        entry["weaknesses"].append(weakness)

        # Extract structured suggestions
        for s in ev.get("improvement_suggestions", []):
            priority = s.get("priority", "medium")
            target = s.get("target", "")
            text = s.get("suggestion", "")
            before = s.get("example_before", "")
            after = s.get("example_after", "")

            suggestion_text = text
            if before and after:
                suggestion_text += f"\n  例: 「{before}」→「{after}」"

            entry["suggestions"].append({
                "priority": priority,
                "target": target,
                "text": suggestion_text,
            })

        entry["scenarios"].append(r.get("scenario_id", ""))

    return by_situation


def format_suggestions_for_optimizer(suggestions: list[dict]) -> list[str]:
    """Convert structured suggestions to flat strings, prioritized."""
    # Sort: high > medium > low
    priority_order = {"high": 0, "medium": 1, "low": 2}
    sorted_suggestions = sorted(suggestions, key=lambda s: priority_order.get(s["priority"], 9))

    # Deduplicate by text similarity (exact match)
    seen = set()
    deduped = []
    for s in sorted_suggestions:
        key = s["text"][:80]
        if key not in seen:
            seen.add(key)
            deduped.append(s)

    return [
        f"[{s['priority']}][{s['target']}] {s['text']}" for s in deduped[:15]
    ]


def print_diff(old: str, new: str, name: str) -> None:
    """Print a unified diff between old and new prompt."""
    old_lines = old.splitlines(keepends=True)
    new_lines = new.splitlines(keepends=True)
    diff = difflib.unified_diff(old_lines, new_lines, fromfile=f"{name} (current)", tofile=f"{name} (improved)")
    diff_str = "".join(diff)
    if diff_str:
        print(diff_str)
    else:
        print("  (no changes)")


def print_situation_summary(sit: str, agg: dict[str, Any]) -> None:
    """Print evaluation summary for a situation."""
    scores = agg["scores"]
    print(f"\n{'='*70}")
    print(f"  Situation: {sit}  ({len(agg['scenarios'])} scenarios)")
    print(f"{'='*70}")

    for dim in ["frontend_fit", "insight_depth", "actionability", "efficiency", "japanese_quality", "user_value"]:
        vals = scores.get(dim, [])
        if vals:
            avg = sum(vals) / len(vals)
            bar = "\u2588" * int(avg) + "\u2591" * (5 - int(avg))
            print(f"  {dim:20s} {avg:4.1f}/5 {bar}")

    if agg["weaknesses"]:
        print(f"\n  Weaknesses ({len(agg['weaknesses'])}):")
        for w in agg["weaknesses"][:8]:
            print(f"    - {w[:120]}")

    suggestions = format_suggestions_for_optimizer(agg["suggestions"])
    if suggestions:
        print(f"\n  Top suggestions ({len(suggestions)}):")
        for s in suggestions[:5]:
            print(f"    - {s[:120]}")


# =========================================================================
# Main
# =========================================================================


async def run_improvement(
    input_path: str,
    situation_filter: str | None = None,
) -> dict:
    """Run the prompt improvement pipeline."""

    # 1. Load evaluation results
    logger.info("Loading evaluation results from %s", input_path)
    results = load_evaluation_results(input_path)
    logger.info("Loaded %d scenario results", len(results))

    # 2. Aggregate by situation
    aggregated = aggregate_by_situation(results)

    if situation_filter:
        aggregated = {k: v for k, v in aggregated.items() if k == situation_filter}

    if not aggregated:
        logger.warning("No evaluation results to process")
        return {"error": "no results"}

    # Print evaluation summary
    for sit, agg in aggregated.items():
        print_situation_summary(sit, agg)

    # 3. Get JWT and fetch current prompts
    logger.info("Logging in...")
    token = await get_jwt_token()

    prompts_by_situation: dict[str, dict] = {}
    for sit in aggregated:
        logger.info("Fetching prompts for situation: %s", sit)
        prompts = await fetch_prompts(token, situation=sit)
        # Pick the default/active prompt
        for p in prompts:
            if p.get("is_default") and p.get("is_active"):
                prompts_by_situation[sit] = p
                break
        if sit not in prompts_by_situation and prompts:
            prompts_by_situation[sit] = prompts[0]

    # 4. Fetch persona prompt (for context)
    persona_prompt = None
    persona_prompts = await fetch_prompts(token, situation="persona")
    for p in persona_prompts:
        if p.get("is_default") and p.get("is_active"):
            persona_prompt = p.get("system_prompt")
            break

    # 5. Generate improved prompts
    optimizer = PromptOptimizer()
    improvement_results: list[dict] = []

    for sit, agg in aggregated.items():
        prompt_data = prompts_by_situation.get(sit)
        if not prompt_data:
            logger.warning("No prompt found for situation: %s, skipping", sit)
            continue

        current_prompt = prompt_data["system_prompt"]
        context_id = prompt_data["id"]
        context_name = prompt_data["name"]

        weaknesses = agg["weaknesses"]
        suggestions = format_suggestions_for_optimizer(agg["suggestions"])

        if not weaknesses and not suggestions:
            logger.info("No weaknesses/suggestions for %s, skipping", sit)
            continue

        logger.info("Generating improved prompt for: %s (%s)", context_name, sit)
        result = await optimizer.generate_improved_prompt(
            current_prompt=current_prompt,
            weaknesses=weaknesses,
            suggestions=suggestions,
            persona_prompt=persona_prompt,
        )

        if result is None:
            logger.warning("Optimization failed for %s", sit)
            improvement_results.append({
                "situation": sit,
                "context_id": context_id,
                "context_name": context_name,
                "status": "failed",
            })
            continue

        improved_prompt = result["improved_prompt"]
        changelog = result["changelog"]
        changes_summary = result["changes_summary"]

        # Print results
        print(f"\n{'#'*70}")
        print(f"  IMPROVED PROMPT: {context_name} ({sit})")
        print(f"{'#'*70}")
        print(f"\n  Changelog: {changelog}")
        print(f"  Changes: {changes_summary}")
        print(f"  Length: {len(current_prompt)} -> {len(improved_prompt)} chars "
              f"({len(improved_prompt)/len(current_prompt)*100:.0f}%)")

        print(f"\n--- Diff ---")
        print_diff(current_prompt, improved_prompt, context_name)

        improvement_results.append({
            "situation": sit,
            "context_id": context_id,
            "context_name": context_name,
            "status": "improved",
            "changelog": changelog,
            "changes_summary": changes_summary,
            "current_prompt_length": len(current_prompt),
            "improved_prompt_length": len(improved_prompt),
            "improved_prompt": improved_prompt,
        })

    # 6. Save results
    output_path = os.getenv("IMPROVEMENT_OUTPUT", DEFAULT_OUTPUT)
    output_data = {
        "evaluation_input": input_path,
        "situations_processed": list(aggregated.keys()),
        "improvements": improvement_results,
    }

    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(output_data, f, ensure_ascii=False, indent=2, default=str)

    print(f"\n{'#'*70}")
    print(f"  SUMMARY")
    print(f"{'#'*70}")
    total = len(improvement_results)
    improved = sum(1 for r in improvement_results if r["status"] == "improved")
    failed = sum(1 for r in improvement_results if r["status"] == "failed")
    print(f"  Total situations: {total}")
    print(f"  Improved: {improved}")
    print(f"  Failed: {failed}")
    print(f"\n  Full results saved to: {output_path}")

    return output_data


def main():
    parser = argparse.ArgumentParser(description="Generate improved prompts from evaluation results")
    parser.add_argument(
        "--input",
        type=str,
        default=DEFAULT_INPUT,
        help=f"Path to evaluation results JSON (default: {DEFAULT_INPUT})",
    )
    parser.add_argument(
        "--situation",
        type=str,
        choices=["chat", "daily_advice", "review"],
        help="Only improve this situation",
    )
    args = parser.parse_args()

    asyncio.run(
        run_improvement(
            input_path=args.input,
            situation_filter=args.situation,
        )
    )


if __name__ == "__main__":
    main()
