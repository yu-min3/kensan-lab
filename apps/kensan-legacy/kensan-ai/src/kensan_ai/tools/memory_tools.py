"""Memory tools for user personalization."""

from datetime import datetime, timedelta
from typing import Any
from zoneinfo import ZoneInfo

from kensan_ai.tools.base import tool
from kensan_ai.db.connection import get_connection
from kensan_ai.lib.parsers import parse_uuid


@tool(
    category="memory",
    name="get_user_memory",
    description="ユーザーのプロフィールサマリー、好み、強み、成長領域を取得する。",
    input_schema={
        "properties": {},
        "required": [],
    },
)
async def get_user_memory(args: dict[str, Any]) -> dict[str, Any]:
    """Get user's memory profile."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    async with get_connection() as conn:
        memory = await conn.fetchrow(
            """
            SELECT profile_summary, preferences, strengths, growth_areas, last_updated
            FROM user_memory
            WHERE user_id = $1
            """,
            user_id,
        )

        if memory:
            return {
                "profileSummary": memory["profile_summary"],
                "preferences": dict(memory["preferences"]) if memory["preferences"] else {},
                "strengths": list(memory["strengths"]) if memory["strengths"] else [],
                "growthAreas": list(memory["growth_areas"]) if memory["growth_areas"] else [],
                "lastUpdated": memory["last_updated"].isoformat() if memory["last_updated"] else None,
            }
        else:
            return {
                "profileSummary": None,
                "preferences": {},
                "strengths": [],
                "growthAreas": [],
                "lastUpdated": None,
            }


@tool(
    category="memory",
    name="get_user_facts",
    description="ユーザーに関する事実（好み、目標、強み、課題など）を取得する。",
    input_schema={
        "properties": {
            "fact_type": {
                "type": "string",
                "description": "ファクトの種類でフィルタ (preference, goal, strength, challenge, schedule, context)",
            },
            "limit": {
                "type": "integer",
                "description": "取得件数上限 (デフォルト: 20)",
            },
        },
        "required": [],
    },
)
async def get_user_facts(args: dict[str, Any]) -> dict[str, Any]:
    """Get facts about a user."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    fact_type = args.get("fact_type")
    # Cap limit to prevent abuse (max 100)
    limit = min(args.get("limit", 20), 100)

    async with get_connection() as conn:
        conditions = ["user_id = $1", "(expires_at IS NULL OR expires_at > NOW())"]
        params: list[Any] = [user_id]
        param_idx = 2

        if fact_type:
            conditions.append(f"fact_type = ${param_idx}")
            params.append(fact_type)
            param_idx += 1

        # Add limit as a parameterized value
        params.append(limit)
        limit_param = f"${param_idx}"

        where_clause = " AND ".join(conditions)

        facts = await conn.fetch(
            f"""
            SELECT id, fact_type, content, source, confidence, expires_at, created_at
            FROM user_facts
            WHERE {where_clause}
            ORDER BY confidence DESC, created_at DESC
            LIMIT {limit_param}
            """,
            *params,
        )

        return {
            "facts": [
                {
                    "id": str(fact["id"]),
                    "factType": fact["fact_type"],
                    "content": fact["content"],
                    "source": fact["source"],
                    "confidence": fact["confidence"],
                    "expiresAt": fact["expires_at"].isoformat() if fact["expires_at"] else None,
                    "createdAt": fact["created_at"].isoformat(),
                }
                for fact in facts
            ]
        }


@tool(
    category="memory",
    name="add_user_fact",
    description="ユーザーに関する新しい事実を記録する。会話中に明示的に言及されたことのみ記録すること。",
    readonly=False,
    input_schema={
        "properties": {
            "fact_type": {
                "type": "string",
                "description": "ファクトの種類 (preference, goal, strength, challenge, schedule, context)",
                "enum": ["preference", "goal", "strength", "challenge", "schedule", "context"],
            },
            "content": {
                "type": "string",
                "description": "ファクトの内容",
            },
            "confidence": {
                "type": "number",
                "description": "確信度 (0.0-1.0、デフォルト: 1.0)",
            },
            "expires_days": {
                "type": "integer",
                "description": "有効期限（日数、省略時は無期限）",
            },
        },
        "required": ["fact_type", "content"],
    },
)
async def add_user_fact(args: dict[str, Any]) -> dict[str, Any]:
    """Add a new fact about a user."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    fact_type = args.get("fact_type")
    content = args.get("content")
    if not fact_type or not content:
        return {"error": "Missing fact_type or content"}

    confidence = args.get("confidence", 1.0)
    expires_days = args.get("expires_days")

    expires_at = None
    if expires_days:
        expires_at = datetime.now(ZoneInfo("UTC")) + timedelta(days=expires_days)

    async with get_connection() as conn:
        fact = await conn.fetchrow(
            """
            INSERT INTO user_facts (user_id, fact_type, content, source, confidence, expires_at)
            VALUES ($1, $2, $3, 'conversation', $4, $5)
            RETURNING id, fact_type, content, source, confidence, expires_at, created_at
            """,
            user_id,
            fact_type,
            content,
            confidence,
            expires_at,
        )

        return {
            "fact": {
                "id": str(fact["id"]),
                "factType": fact["fact_type"],
                "content": fact["content"],
                "source": fact["source"],
                "confidence": fact["confidence"],
                "expiresAt": fact["expires_at"].isoformat() if fact["expires_at"] else None,
                "createdAt": fact["created_at"].isoformat(),
            }
        }


@tool(
    category="memory",
    name="get_recent_interactions",
    description="ユーザーの最近のAIとの会話履歴を取得する。",
    input_schema={
        "properties": {
            "limit": {
                "type": "integer",
                "description": "取得件数上限 (デフォルト: 5)",
            },
            "situation": {
                "type": "string",
                "description": "状況でフィルタ (chat, morning, evening, weekly)",
            },
        },
        "required": [],
    },
)
async def get_recent_interactions(args: dict[str, Any]) -> dict[str, Any]:
    """Get user's recent AI interactions."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    # Cap limit to prevent abuse (max 100)
    limit = min(args.get("limit", 5), 100)
    situation = args.get("situation")

    async with get_connection() as conn:
        conditions = ["user_id = $1"]
        params: list[Any] = [user_id]
        param_idx = 2

        if situation:
            conditions.append(f"situation = ${param_idx}")
            params.append(situation)
            param_idx += 1

        # Add limit as a parameterized value
        params.append(limit)
        limit_param = f"${param_idx}"

        where_clause = " AND ".join(conditions)

        interactions = await conn.fetch(
            f"""
            SELECT id, session_id, situation, user_input, ai_output, created_at
            FROM ai_interactions
            WHERE {where_clause}
            ORDER BY created_at DESC
            LIMIT {limit_param}
            """,
            *params,
        )

        return {
            "interactions": [
                {
                    "id": str(interaction["id"]),
                    "sessionId": str(interaction["session_id"]),
                    "situation": interaction["situation"],
                    "userInput": interaction["user_input"],
                    "aiOutput": interaction["ai_output"][:500] + "..." if len(interaction["ai_output"]) > 500 else interaction["ai_output"],
                    "createdAt": interaction["created_at"].isoformat(),
                }
                for interaction in interactions
            ]
        }


# All memory tools for export
ALL_MEMORY_TOOLS = [
    get_user_memory,
    get_user_facts,
    add_user_fact,
    get_recent_interactions,
]
