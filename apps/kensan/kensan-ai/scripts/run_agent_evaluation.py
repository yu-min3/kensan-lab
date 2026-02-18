#!/usr/bin/env python3
"""Run agent evaluator against the live kensan-ai service.

Usage:
    cd kensan-ai
    uv run python scripts/run_agent_evaluation.py

    # Specific situation only:
    uv run python scripts/run_agent_evaluation.py --situation chat

    # Specific scenario only:
    uv run python scripts/run_agent_evaluation.py --scenario chat_create_03

    # Limit scenarios (quick smoke test):
    uv run python scripts/run_agent_evaluation.py --limit 3
"""

from __future__ import annotations

import argparse
import asyncio
import json
import logging
import os
import sys
import time

import httpx

# Add src to path so we can import kensan_ai modules
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from kensan_ai.batch.agent_evaluator import (
    EVALUATOR_SYSTEM_PROMPT,
    TEST_SCENARIOS,
    AgentEvaluator,
)

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(message)s",
)
logger = logging.getLogger(__name__)

BASE_URL = os.getenv("KENSAN_AI_URL", "http://kensan-ai-service:8089/api/v1")
AUTH_URL = os.getenv("KENSAN_AUTH_URL", "http://user-service:8081")
TEST_EMAIL = "test@kensan.dev"
TEST_PASSWORD = "password123"


async def get_jwt_token() -> str:
    """Login and get JWT token."""
    async with httpx.AsyncClient() as client:
        resp = await client.post(
            f"{AUTH_URL}/api/v1/auth/login",
            json={"email": TEST_EMAIL, "password": TEST_PASSWORD},
        )
        resp.raise_for_status()
        return resp.json()["data"]["token"]


async def call_agent_stream(
    message: str,
    situation: str,
    token: str,
    timeout: float = 120.0,
) -> tuple[str, list[dict], list[dict]]:
    """Call /agent/stream and parse SSE events.

    Returns:
        (text, tool_calls, action_proposals)
    """
    text_parts: list[str] = []
    tool_calls: list[dict] = []
    action_proposals: list[dict] = []

    headers = {"Authorization": f"Bearer {token}"}
    payload = {
        "message": message,
        "situation": situation,
    }

    async with httpx.AsyncClient(timeout=httpx.Timeout(timeout)) as client:
        async with client.stream(
            "POST",
            f"{BASE_URL}/agent/stream",
            json=payload,
            headers=headers,
        ) as response:
            response.raise_for_status()

            buffer = ""
            async for chunk in response.aiter_text():
                buffer += chunk
                while "\n\n" in buffer:
                    block, buffer = buffer.split("\n\n", 1)
                    event_type, event_data = _parse_sse_block(block)
                    if event_type is None:
                        continue

                    if event_type == "text":
                        text_parts.append(event_data.get("content", ""))
                    elif event_type == "tool_call":
                        tool_calls.append({
                            "id": event_data.get("id"),
                            "name": event_data.get("name"),
                        })
                    elif event_type == "tool_result":
                        # Mark tool as completed
                        tid = event_data.get("id")
                        for tc in tool_calls:
                            if tc["id"] == tid:
                                tc["completed"] = True
                    elif event_type == "action_proposal":
                        for action in event_data.get("actions", []):
                            action_proposals.append({
                                "tool_name": action.get("tool_name", ""),
                                "description": action.get("description", ""),
                                "input": action.get("input", {}),
                            })
                    elif event_type == "error":
                        logger.error("Agent error: %s", event_data.get("message"))
                    elif event_type == "done":
                        pass  # stream complete

    full_text = "".join(text_parts)
    return full_text, tool_calls, action_proposals


def _parse_sse_block(block: str) -> tuple[str | None, dict]:
    """Parse a single SSE block into (event_type, data_dict)."""
    event_type = None
    data_lines: list[str] = []

    for line in block.strip().split("\n"):
        if line.startswith("event:"):
            event_type = line[6:].strip()
        elif line.startswith("data:"):
            data_lines.append(line[5:].strip())

    if not event_type or not data_lines:
        return None, {}

    data_str = "\n".join(data_lines)
    try:
        data = json.loads(data_str)
    except json.JSONDecodeError:
        data = {"raw": data_str}

    return event_type, data


def print_scenario_result(scenario_id: str, message: str, evaluation: dict) -> None:
    """Pretty-print a single scenario evaluation."""
    scores = evaluation.get("scores", {})
    violations = evaluation.get("checklist_violations", [])
    suggestions = evaluation.get("improvement_suggestions", [])
    overall = evaluation.get("overall_assessment", "")

    # Score summary line
    score_parts = []
    for dim in [
        "frontend_fit",
        "insight_depth",
        "actionability",
        "efficiency",
        "japanese_quality",
        "user_value",
    ]:
        if dim in scores and isinstance(scores[dim], dict):
            s = scores[dim].get("score", "?")
            score_parts.append(f"{dim}={s}")

    avg = None
    numeric_scores = []
    for dim_data in scores.values():
        if isinstance(dim_data, dict) and isinstance(dim_data.get("score"), (int, float)):
            numeric_scores.append(dim_data["score"])
    if numeric_scores:
        avg = sum(numeric_scores) / len(numeric_scores)

    print(f"\n{'='*70}")
    print(f"  {scenario_id}: {message}")
    print(f"{'='*70}")
    print(f"  Scores: {', '.join(score_parts)}")
    if avg is not None:
        print(f"  Average: {avg:.1f}/5")
    print(f"  Overall: {overall}")

    if violations:
        print(f"\n  Violations:")
        for v in violations:
            print(f"    - {v}")

    if suggestions:
        print(f"\n  Suggestions:")
        for s in suggestions:
            priority = s.get("priority", "?")
            target = s.get("target", "?")
            text = s.get("suggestion", "")
            print(f"    [{priority}] ({target}) {text}")
            if s.get("example_before"):
                print(f"      Before: {s['example_before'][:100]}")
            if s.get("example_after"):
                print(f"      After:  {s['example_after'][:100]}")


async def run_evaluation(
    situations: list[str] | None = None,
    scenario_id: str | None = None,
    limit: int | None = None,
) -> dict:
    """Run the full evaluation pipeline."""
    # 1. Get JWT token
    logger.info("Logging in as %s...", TEST_EMAIL)
    token = await get_jwt_token()
    logger.info("Got JWT token")

    # 2. Build scenario list
    scenarios: list[tuple[str, dict]] = []
    target_situations = situations or list(TEST_SCENARIOS.keys())

    for sit in target_situations:
        for sc in TEST_SCENARIOS.get(sit, []):
            if scenario_id and sc["id"] != scenario_id:
                continue
            scenarios.append((sit, sc))

    if limit:
        scenarios = scenarios[:limit]

    logger.info("Running %d scenarios across %s", len(scenarios), target_situations)

    # 3. Initialize evaluator
    evaluator = AgentEvaluator()

    # 4. Run each scenario
    all_results: list[dict] = []

    for i, (sit, sc) in enumerate(scenarios, 1):
        msg = sc["message"]
        logger.info("[%d/%d] %s: \"%s\"", i, len(scenarios), sc["id"], msg)

        # Call the agent
        t0 = time.time()
        try:
            text, tool_calls, proposals = await call_agent_stream(
                message=msg,
                situation=sit,
                token=token,
            )
            elapsed = time.time() - t0
            logger.info(
                "  Agent responded in %.1fs (text=%d chars, tools=%d, proposals=%d)",
                elapsed,
                len(text),
                len(tool_calls),
                len(proposals),
            )
        except Exception as e:
            logger.error("  Agent call failed: %s", e)
            all_results.append({
                "scenario_id": sc["id"],
                "error": str(e),
            })
            continue

        # Log agent response summary
        if text:
            logger.info("  Text: %s...", text[:120].replace("\n", " "))
        if tool_calls:
            tool_names = [tc["name"] for tc in tool_calls]
            logger.info("  Tools: %s", ", ".join(tool_names))
        if proposals:
            prop_names = [p["tool_name"] for p in proposals]
            logger.info("  Proposals: %s", ", ".join(prop_names))

        # Evaluate
        logger.info("  Evaluating...")
        t0 = time.time()
        evaluation = await evaluator.evaluate_response(
            situation=sit,
            test_message=msg,
            agent_response=text,
            tool_calls=tool_calls,
            action_proposals=proposals,
        )
        eval_elapsed = time.time() - t0
        logger.info("  Evaluation done in %.1fs", eval_elapsed)

        result = {
            "scenario_id": sc["id"],
            "situation": sit,
            "message": msg,
            "agent_text": text,
            "agent_tool_calls": tool_calls,
            "agent_proposals": proposals,
            "evaluation": evaluation,
        }
        all_results.append(result)

        # Print result
        print_scenario_result(sc["id"], msg, evaluation)

    # 5. Print summary
    print(f"\n{'#'*70}")
    print(f"  SUMMARY")
    print(f"{'#'*70}")

    dim_scores: dict[str, list[float]] = {}
    for r in all_results:
        ev = r.get("evaluation", {})
        scores = ev.get("scores", {})
        for dim, data in scores.items():
            if isinstance(data, dict) and isinstance(data.get("score"), (int, float)):
                dim_scores.setdefault(dim, []).append(data["score"])

    print(f"\n  Dimension averages ({len(all_results)} scenarios):")
    total_avg_parts = []
    for dim in [
        "frontend_fit",
        "insight_depth",
        "actionability",
        "efficiency",
        "japanese_quality",
        "user_value",
    ]:
        vals = dim_scores.get(dim, [])
        if vals:
            avg = sum(vals) / len(vals)
            total_avg_parts.append(avg)
            bar = "█" * int(avg) + "░" * (5 - int(avg))
            print(f"    {dim:20s} {avg:4.1f}/5 {bar}")

    if total_avg_parts:
        overall = sum(total_avg_parts) / len(total_avg_parts)
        print(f"\n    {'OVERALL':20s} {overall:4.1f}/5")

    # Collect all high-priority suggestions
    high_suggestions: list[str] = []
    for r in all_results:
        ev = r.get("evaluation", {})
        for s in ev.get("improvement_suggestions", []):
            if s.get("priority") == "high":
                high_suggestions.append(s.get("suggestion", ""))

    if high_suggestions:
        print(f"\n  High-priority improvements:")
        for s in list(set(high_suggestions))[:5]:
            print(f"    - {s}")

    # 6. Save full results
    output_path = os.getenv(
        "EVAL_OUTPUT", os.path.join("/tmp", "evaluation_results.json")
    )
    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(all_results, f, ensure_ascii=False, indent=2, default=str)
    print(f"\n  Full results saved to: {output_path}")

    return {"results": all_results, "output_path": output_path}


def main():
    parser = argparse.ArgumentParser(description="Run AI agent evaluation")
    parser.add_argument(
        "--situation",
        type=str,
        choices=["chat", "daily_advice", "review"],
        help="Only test this situation",
    )
    parser.add_argument(
        "--scenario",
        type=str,
        help="Only run a specific scenario by ID (e.g., chat_create_03)",
    )
    parser.add_argument(
        "--limit",
        type=int,
        help="Limit number of scenarios to run",
    )
    args = parser.parse_args()

    situations = [args.situation] if args.situation else None

    asyncio.run(
        run_evaluation(
            situations=situations,
            scenario_id=args.scenario,
            limit=args.limit,
        )
    )


if __name__ == "__main__":
    main()
