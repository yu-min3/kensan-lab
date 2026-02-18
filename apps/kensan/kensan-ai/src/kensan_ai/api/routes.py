"""API routes for kensan-ai."""

import asyncio
import json
import logging
import random
import time as time_module
import uuid as uuid_module
from datetime import date, datetime, timezone
from typing import Any
from uuid import UUID

import jwt
from fastapi import APIRouter, HTTPException, Header, Query
from fastapi.responses import StreamingResponse

from kensan_ai.agents import create_agent_runner
from kensan_ai.agents.conversation_store import (
    ConversationStore,
    ConversationState,
    PendingAction,
)
from kensan_ai.config import get_settings
from kensan_ai.api.schemas import (
    HealthResponse,
    FeedbackRequest,
    FeedbackResponse,
    AgentStreamRequest,
    AgentApproveRequest,
    AgentRejectRequest,
    AIContextResponse,
    AIContextUpdateRequest,
    AIContextVersionResponse,
    PromptMetadataResponse,
    VariableMetadataItem,
    ToolMetadataItem,
    ConversationRateRequest,
)
from kensan_ai.api.sse import sse_event
from kensan_ai.agents.chat import select_tools, get_deferred_write_tools
from kensan_ai.context import Situation, ContextResolver, detect_situation
from kensan_ai.db.connection import get_connection
from kensan_ai.tools import execute_tool
from kensan_ai.logging import InteractionLogger
from kensan_ai.extraction.fact_extractor import get_fact_extractor
from kensan_ai.batch.profile_summarizer import ProfileSummarizer
from kensan_ai.db.queries import interactions as interactions_queries
from kensan_ai.db.queries import ai_contexts as ai_contexts_queries
from kensan_ai.lib.parsers import parse_uuid as lib_parse_uuid, parse_date as lib_parse_date
from kensan_ai.lakehouse import get_reader
from kensan_ai.telemetry import get_tracer

_tracer = get_tracer("kensan-ai.routes")

logger = logging.getLogger(__name__)

router = APIRouter()

# Singleton conversation store for agent approval flow
conversation_store = ConversationStore(ttl_minutes=30)


def _parse_uuid(value: str) -> UUID:
    """Parse a string to UUID, raising HTTPException if invalid."""
    result = lib_parse_uuid(value)
    if result is None:
        raise HTTPException(status_code=400, detail=f"Invalid UUID: {value}")
    return result


def _parse_date(value: str) -> date:
    """Parse a date string (YYYY-MM-DD), raising HTTPException if invalid."""
    result = lib_parse_date(value)
    if result is None:
        raise HTTPException(status_code=400, detail=f"Invalid date: {value}")
    return result


def _get_user_id_from_header(authorization: str | None) -> UUID:
    """Extract user ID from Authorization header (JWT token)."""
    if not authorization:
        raise HTTPException(status_code=401, detail="Authorization header required")

    if not authorization.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="Invalid authorization format")

    token = authorization[7:]

    # First, try to decode as JWT
    try:
        settings = get_settings()
        payload = jwt.decode(
            token,
            settings.jwt_secret,
            algorithms=["HS256"],
            options={"verify_exp": True}
        )
        user_id = payload.get("user_id") or payload.get("sub")
        if user_id:
            return _parse_uuid(user_id)
    except jwt.ExpiredSignatureError:
        raise HTTPException(status_code=401, detail="Token has expired")
    except jwt.InvalidTokenError:
        pass  # Fall through to other methods

    # Fallback: only in development
    if get_settings().server_env != "production":
        if token.startswith("user_id:"):
            return _parse_uuid(token[8:])
        try:
            return UUID(token)
        except ValueError:
            pass

    raise HTTPException(status_code=401, detail="Invalid token")


# =============================================================================
# Health Check
# =============================================================================

@router.get("/health", response_model=HealthResponse)
async def health_check() -> HealthResponse:
    """Health check endpoint."""
    return HealthResponse()


# =============================================================================
# Post-Stream Pipeline
# =============================================================================


async def _post_stream_pipeline(
    user_id: UUID,
    situation: str,
    user_input: str,
    ai_output: str,
    conv_id: str,
    context_id: UUID | None,
    persona_context_id: UUID | None,
    tokens_input: int,
    tokens_output: int,
    latency_ms: int,
) -> None:
    """Background pipeline after stream completes: log, extract facts, update profile."""
    try:
        # 1. Log interaction
        conv_uuid: UUID | None = None
        try:
            conv_uuid = UUID(conv_id)
        except ValueError:
            pass

        interaction_id = await InteractionLogger.log(
            user_id=user_id,
            session_id=user_id,
            situation=situation,
            user_input=user_input,
            ai_output=ai_output,
            tool_calls=None,
            tokens_input=tokens_input,
            tokens_output=tokens_output,
            latency_ms=latency_ms,
            context_id=context_id,
            conversation_id=conv_uuid,
            persona_context_id=persona_context_id,
        )

        # 2. Extract and save facts
        extractor = get_fact_extractor()
        saved = await extractor.extract_and_save(
            user_id=user_id,
            user_input=user_input,
            ai_output=ai_output,
            interaction_id=interaction_id,
        )

        # 3. Update profile if new facts were found
        if saved > 0:
            summarizer = ProfileSummarizer()
            await summarizer.summarize_user(user_id)

    except Exception as e:
        logger.error(f"Post-stream pipeline error: {e}")


# =============================================================================
# Unified Agent Endpoints
# =============================================================================

@router.post("/agent/stream")
async def agent_stream(
    request: AgentStreamRequest,
    authorization: str | None = Header(None),
) -> StreamingResponse:
    """Unified agent streaming endpoint with SSE.

    Handles all AI interactions through a single endpoint.
    Read-only tool calls are executed immediately.
    Write tool calls are collected and returned as action proposals.
    """
    user_id = _get_user_id_from_header(authorization)

    # 1-2. Detect situation and resolve context
    with _tracer.start_as_current_span("agent.context_resolution") as ctx_span:
        ctx_span.set_attribute("gen_ai.user.id", str(user_id))

        if request.context_id:
            # Direct context_id: load by ID (used by A/B comparison)
            direct_ctx_id = _parse_uuid(request.context_id)
            context = await ContextResolver.get_context_by_id(
                direct_ctx_id,
                user_id=user_id,
                version_number=request.version_number,
            )
            if not context:
                raise HTTPException(
                    status_code=404,
                    detail=f"AI context not found: {request.context_id}",
                )
            situation = context.situation
            ctx_span.set_attribute("gen_ai.request.situation", situation.value)
        else:
            situation = detect_situation(
                explicit_situation=request.situation if request.situation != "auto" else None,
            )
            ctx_span.set_attribute("gen_ai.request.situation", situation.value)

            context = await ContextResolver.get_context(situation, user_id=user_id)

            if not context:
                logger.error(f"No AI context found for situation={situation.value}")
                raise HTTPException(
                    status_code=500,
                    detail=f"AI context not configured for situation: {situation.value}",
                )

        ctx_span.set_attribute("gen_ai.request.context_name", context.name if hasattr(context, "name") else situation.value)

    system_prompt = context.system_prompt
    context_keys: list[str] = []
    # フロントから渡された既存データをシステムプロンプトに注入
    if request.context:
        context_keys = list(request.context.keys())
        context_lines = []
        for key, value in request.context.items():
            context_lines.append(f"### {key}\n{value}")
        system_prompt += (
            "\n\n## 提供済みデータ（ツールで再取得不要）\n"
            "以下のデータは取得済みです。このデータを直接使い、"
            "不足するデータだけをツールで取得してください。"
            "複数のツールが必要な場合は1回のターンでまとめて呼び出してください。\n\n"
            + "\n\n".join(context_lines)
        )
    max_turns = context.max_turns
    temperature = context.temperature

    # 3. Get or create conversation state
    conv_id = request.conversation_id or str(uuid_module.uuid4())
    state = conversation_store.get(conv_id)
    if state is None:
        state = ConversationState(
            conversation_id=conv_id,
            user_id=user_id,
        )

    # 3.5. If there are pending (unapproved) actions from the previous turn,
    # the user chose to type a new message instead of approving/rejecting.
    # Record this as an implicit rejection so the agent treats the new message
    # as a modification request rather than repeating the same proposal.
    continuing_proposal = bool(state.pending_actions)
    if state.pending_actions:
        action_descs = [a.tool_name for a in state.pending_actions]
        state.message_history.add_assistant_message([
            {
                "type": "text",
                "text": (
                    f"（前回の提案（{', '.join(action_descs)}）はユーザーが承認せず、"
                    "代わりに新しいメッセージを送信しました。"
                    "前回の提案は破棄されています。ユーザーの次のメッセージは修正依頼の可能性が高いです。）"
                ),
            }
        ])
        state.pending_actions.clear()
        conversation_store.set(state)

    # 3.6. Select tools based on message intent.
    deferred_tools: list[str] | None = None
    if continuing_proposal:
        allowed_tools = list(context.allowed_tools)
    else:
        allowed_tools = select_tools(
            request.message, context.allowed_tools, request.situation, context_keys,
            prompt_variables=context.prompt_variables,
        )
        # Compute deferred write tools: these will be unlocked after readonly tool execution
        deferred_tools = get_deferred_write_tools(allowed_tools, context.allowed_tools) or None

    # 4. Create agent runner (Anthropic or Gemini based on ai_provider setting)
    agent = create_agent_runner(
        system_prompt=system_prompt,
        allowed_tools=allowed_tools,
        max_turns=max_turns,
        temperature=temperature,
        context_id=str(context.id),
        context_name=context.name,
        context_version=context.version,
        deferred_tools=deferred_tools,
    )

    # 5. Stream SSE events
    async def event_generator():
        pending_actions: list[PendingAction] = []
        collected_text: list[str] = []
        tokens_input = 0
        tokens_output = 0
        start_time = time_module.monotonic()

        async for event_str in agent.stream_sse(
            user_message=request.message,
            user_id=str(user_id),
            conversation_id=conv_id,
            history=state.message_history,
        ):
            yield event_str

            # Parse SSE events to collect data for post-stream pipeline
            if event_str.startswith("event: text"):
                try:
                    data_line = event_str.split("\ndata: ", 1)[1].rstrip("\n")
                    data = json.loads(data_line)
                    collected_text.append(data.get("content", ""))
                except (IndexError, json.JSONDecodeError):
                    pass

            elif event_str.startswith("event: action_proposal"):
                data_line = event_str.split("\ndata: ", 1)[1].rstrip("\n")
                data = json.loads(data_line)
                for action in data.get("actions", []):
                    pending_actions.append(PendingAction(
                        id=action["id"],
                        tool_name=action["tool_name"],
                        description=action["description"],
                        input=action["input"],
                    ))

            elif event_str.startswith("event: done"):
                try:
                    data_line = event_str.split("\ndata: ", 1)[1].rstrip("\n")
                    data = json.loads(data_line)
                    tokens = data.get("tokens", {})
                    tokens_input = tokens.get("input", 0)
                    tokens_output = tokens.get("output", 0)
                except (IndexError, json.JSONDecodeError):
                    pass

        # Save state with pending actions
        state.pending_actions = pending_actions
        conversation_store.set(state)

        # Launch post-stream pipeline in background
        ai_output = "".join(collected_text)
        latency_ms = int((time_module.monotonic() - start_time) * 1000)
        if ai_output:
            asyncio.create_task(
                _post_stream_pipeline(
                    user_id=user_id,
                    situation=situation.value,
                    user_input=request.message,
                    ai_output=ai_output,
                    conv_id=conv_id,
                    context_id=context.id if context else None,
                    persona_context_id=context.persona_context_id if context else None,
                    tokens_input=tokens_input,
                    tokens_output=tokens_output,
                    latency_ms=latency_ms,
                )
            )

    return StreamingResponse(
        event_generator(),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )


@router.post("/agent/approve")
async def agent_approve(
    request: AgentApproveRequest,
    authorization: str | None = Header(None),
) -> StreamingResponse:
    """Execute approved actions from a previous agent proposal."""
    user_id = _get_user_id_from_header(authorization)
    state = conversation_store.get(request.conversation_id)

    if not state or state.user_id != user_id:
        raise HTTPException(status_code=404, detail="Conversation not found")

    approved_ids = set(request.action_ids)
    approved = [a for a in state.pending_actions if a.id in approved_ids]
    rejected = [a for a in state.pending_actions if a.id not in approved_ids]

    async def event_generator():
        success_count = 0
        fail_count = 0
        results_summary: list[str] = []

        for action in approved:
            yield sse_event("tool_call", {
                "id": f"tc_{action.id}",
                "name": action.tool_name,
            })

            try:
                await execute_tool(action.tool_name, action.input)
                yield sse_event("tool_result", {
                    "id": f"tc_{action.id}",
                    "name": action.tool_name,
                })
                success_count += 1
                results_summary.append(f"✓ {action.tool_name}")
            except Exception as e:
                logger.error(f"Action execution failed: {action.tool_name}: {e}")
                yield sse_event("tool_result", {
                    "id": f"tc_{action.id}",
                    "name": action.tool_name,
                    "error": str(e),
                })
                fail_count += 1
                results_summary.append(f"✗ {action.tool_name}: {e}")

        if fail_count == 0:
            yield sse_event("text", {
                "content": f"{success_count}件のアクションを実行しました。",
            })
        else:
            yield sse_event("text", {
                "content": f"{success_count}件成功、{fail_count}件失敗しました。",
            })
        yield sse_event("done", {
            "conversation_id": request.conversation_id,
            "tokens": {"input": 0, "output": 0},
        })

        # Record execution results in conversation history
        history_parts = results_summary.copy()
        if rejected:
            rejected_names = [a.tool_name for a in rejected]
            history_parts.append(
                f"却下: {', '.join(rejected_names)}"
            )
        if history_parts:
            state.message_history.add_assistant_message([
                {"type": "text", "text": "実行結果: " + "; ".join(history_parts)}
            ])

        state.pending_actions.clear()
        conversation_store.set(state)

    return StreamingResponse(
        event_generator(),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )


@router.post("/agent/reject")
async def agent_reject(
    request: AgentRejectRequest,
    authorization: str | None = Header(None),
) -> dict:
    """Record rejection of proposed actions in conversation history."""
    user_id = _get_user_id_from_header(authorization)
    state = conversation_store.get(request.conversation_id)

    if not state or state.user_id != user_id:
        raise HTTPException(status_code=404, detail="Conversation not found")

    if state.pending_actions:
        action_names = [a.tool_name for a in state.pending_actions]
        state.message_history.add_assistant_message([
            {
                "type": "text",
                "text": f"ユーザーは提案を却下しました（{', '.join(action_names)}）。"
                "次回は異なるアプローチで提案してください。",
            }
        ])

    state.pending_actions.clear()
    conversation_store.set(state)

    return {"status": "ok"}


# =============================================================================
# Conversation History
# =============================================================================

@router.get("/conversations")
async def get_conversations(
    authorization: str | None = Header(None),
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
) -> dict:
    """Get list of past conversations."""
    user_id = _get_user_id_from_header(authorization)
    conversations = await interactions_queries.get_conversations(
        user_id=user_id,
        limit=limit,
        offset=offset,
    )
    return {"conversations": conversations}


@router.get("/conversations/{conversation_id}")
async def get_conversation_detail(
    conversation_id: str,
    authorization: str | None = Header(None),
) -> dict:
    """Get messages for a specific conversation."""
    user_id = _get_user_id_from_header(authorization)
    conv_uuid = _parse_uuid(conversation_id)
    messages = await interactions_queries.get_conversation_messages(
        user_id=user_id,
        conversation_id=conv_uuid,
    )
    if not messages:
        raise HTTPException(status_code=404, detail="Conversation not found")
    return {"messages": messages}


# =============================================================================
# Conversation Rating
# =============================================================================

@router.post("/conversations/{conversation_id}/rate")
async def rate_conversation(
    conversation_id: str,
    request: ConversationRateRequest,
    authorization: str | None = Header(None),
) -> dict:
    """Rate a conversation."""
    user_id = _get_user_id_from_header(authorization)
    conv_uuid = _parse_uuid(conversation_id)

    async with get_connection() as conn:
        interaction_row = None
        for attempt in range(3):
            interaction_row = await conn.fetchrow(
                """
                SELECT id FROM ai_interactions
                WHERE conversation_id = $1 AND user_id = $2
                ORDER BY created_at DESC
                LIMIT 1
                """,
                conv_uuid,
                user_id,
            )
            if interaction_row:
                break
            if attempt < 2:
                await asyncio.sleep(1)

    if not interaction_row:
        raise HTTPException(status_code=404, detail="Conversation interaction not found")

    success = await InteractionLogger.add_feedback(
        interaction_id=interaction_row["id"],
        rating=request.rating,
        feedback=request.feedback,
    )

    if not success:
        raise HTTPException(status_code=500, detail="Failed to save rating")

    return {"success": True, "message": "評価を記録しました"}


# =============================================================================
# Feedback (kept for interaction tracking)
# =============================================================================

@router.post("/interactions/{interaction_id}/feedback", response_model=FeedbackResponse)
async def feedback_endpoint(
    interaction_id: str,
    request: FeedbackRequest,
    authorization: str | None = Header(None),
) -> FeedbackResponse:
    """Add feedback to an AI interaction."""
    _get_user_id_from_header(authorization)
    try:
        interaction_uuid = _parse_uuid(interaction_id)
        success = await InteractionLogger.add_feedback(
            interaction_id=interaction_uuid,
            rating=request.rating,
            feedback=request.feedback,
        )

        if success:
            return FeedbackResponse(success=True, message="Feedback recorded")
        else:
            raise HTTPException(status_code=404, detail="Interaction not found")
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))


# =============================================================================
# Prompt Management
# =============================================================================

@router.get("/prompts", response_model=list[AIContextResponse])
async def list_prompts(
    authorization: str | None = Header(None),
    situation: str | None = Query(None),
) -> list[dict]:
    """List all AI contexts (prompts) for the current user."""
    user_id = _get_user_id_from_header(authorization)
    contexts = await ai_contexts_queries.list_contexts(situation=situation, user_id=user_id)
    return contexts


@router.get("/prompts/metadata", response_model=PromptMetadataResponse)
async def get_prompt_metadata(
    authorization: str | None = Header(None),
) -> PromptMetadataResponse:
    """Get metadata for all available variables and tools."""
    _get_user_id_from_header(authorization)

    from kensan_ai.context.variable_replacer import VariableReplacer
    from kensan_ai.tools import get_all_tools

    variables = [
        VariableMetadataItem(**v) for v in VariableReplacer.get_variable_metadata()
    ]

    tools = [
        ToolMetadataItem(
            name=t.name,
            description=t.description,
            readonly=t.readonly,
            category=t.category,
        )
        for t in get_all_tools()
    ]

    return PromptMetadataResponse(variables=variables, tools=tools)


@router.post("/prompts/run-optimization")
async def run_optimization_for_user(
    authorization: str | None = Header(None),
    force: bool = Query(False),
) -> dict:
    """Run prompt optimization for the current user.

    Evaluates conversation quality, generates improved prompts,
    and creates candidate versions. JWT-authenticated (per-user).
    """
    from kensan_ai.batch.experiment_manager import _process_user_contexts, _last_week_range
    from kensan_ai.batch.prompt_evaluator import PromptEvaluator
    from kensan_ai.batch.prompt_optimizer import PromptOptimizer

    user_id = _get_user_id_from_header(authorization)
    period_start, period_end = _last_week_range()

    evaluator = PromptEvaluator()
    optimizer = PromptOptimizer()

    result = await _process_user_contexts(
        user_id=user_id,
        period_start=period_start,
        period_end=period_end,
        evaluator=evaluator,
        optimizer=optimizer,
        force=force,
    )

    return {
        "period_start": period_start.isoformat(),
        "period_end": period_end.isoformat(),
        **result,
    }


@router.get("/prompts/{context_id}", response_model=AIContextResponse)
async def get_prompt(
    context_id: str,
    authorization: str | None = Header(None),
) -> dict:
    """Get a single AI context by ID."""
    _get_user_id_from_header(authorization)
    ctx_uuid = _parse_uuid(context_id)
    context = await ai_contexts_queries.get_context(ctx_uuid)
    if not context:
        raise HTTPException(status_code=404, detail="Context not found")
    return context


@router.patch("/prompts/{context_id}", response_model=AIContextResponse)
async def update_prompt(
    context_id: str,
    request: AIContextUpdateRequest,
    authorization: str | None = Header(None),
) -> dict:
    """Update an AI context and create a new version."""
    _get_user_id_from_header(authorization)
    ctx_uuid = _parse_uuid(context_id)
    result = await ai_contexts_queries.update_context(
        context_id=ctx_uuid,
        system_prompt=request.system_prompt,
        allowed_tools=request.allowed_tools,
        max_turns=request.max_turns,
        temperature=request.temperature,
        changelog=request.changelog,
    )
    if not result:
        raise HTTPException(status_code=404, detail="Context not found")
    return result


@router.get("/prompts/{context_id}/versions", response_model=list[AIContextVersionResponse])
async def list_prompt_versions(
    context_id: str,
    authorization: str | None = Header(None),
) -> list[dict]:
    """List all versions for a context."""
    _get_user_id_from_header(authorization)
    ctx_uuid = _parse_uuid(context_id)
    versions = await ai_contexts_queries.list_versions(ctx_uuid)
    return versions


@router.get("/prompts/{context_id}/versions/{version_number}", response_model=AIContextVersionResponse)
async def get_prompt_version(
    context_id: str,
    version_number: int,
    authorization: str | None = Header(None),
) -> dict:
    """Get a specific version of a context."""
    _get_user_id_from_header(authorization)
    ctx_uuid = _parse_uuid(context_id)
    version = await ai_contexts_queries.get_version(ctx_uuid, version_number)
    if not version:
        raise HTTPException(status_code=404, detail="Version not found")
    return version


@router.post("/prompts/{context_id}/rollback/{version_number}", response_model=AIContextResponse)
async def rollback_prompt(
    context_id: str,
    version_number: int,
    authorization: str | None = Header(None),
) -> dict:
    """Rollback a context to a specific version."""
    _get_user_id_from_header(authorization)
    ctx_uuid = _parse_uuid(context_id)
    result = await ai_contexts_queries.rollback_to_version(ctx_uuid, version_number)
    if not result:
        raise HTTPException(status_code=404, detail="Version not found")
    return result


@router.post("/prompts/{context_id}/versions/{version_number}/adopt")
async def adopt_version(
    context_id: str,
    version_number: int,
    authorization: str | None = Header(None),
) -> dict:
    """Adopt a candidate version as the active version."""
    _get_user_id_from_header(authorization)
    ctx_uuid = _parse_uuid(context_id)
    result = await ai_contexts_queries.adopt_version(ctx_uuid, version_number)
    if not result:
        raise HTTPException(status_code=404, detail="Version not found")
    return {"status": "adopted", "message": "改善版プロンプトが採用されました", "context": result}


@router.post("/prompts/{context_id}/versions/{version_number}/reject")
async def reject_version(
    context_id: str,
    version_number: int,
    authorization: str | None = Header(None),
) -> dict:
    """Reject a candidate version."""
    _get_user_id_from_header(authorization)
    ctx_uuid = _parse_uuid(context_id)
    result = await ai_contexts_queries.reject_version(ctx_uuid, version_number)
    if not result:
        raise HTTPException(status_code=404, detail="Version not found")
    return {"status": "rejected", "message": "改善版プロンプトは却下されました", "context": result}


# =============================================================================
# Admin (Internal, no JWT)
# =============================================================================

@router.post("/admin/reindex-pending")
async def admin_reindex_pending(
    batch_size: int = Query(50, ge=1, le=500),
) -> dict:
    """Reindex all notes with index_status='pending' across all users."""
    from kensan_ai.indexing.pipeline import reindex_pending_notes

    async with get_connection() as conn:
        rows = await conn.fetch(
            "SELECT DISTINCT user_id FROM notes WHERE index_status = 'pending'"
        )

    user_ids = [row["user_id"] for row in rows]
    if not user_ids:
        return {"users_processed": 0, "total_processed": 0, "total_chunks": 0}

    total_processed = 0
    total_chunks = 0
    for uid in user_ids:
        result = await reindex_pending_notes(uid, batch_size=batch_size)
        total_processed += result["processed"]
        total_chunks += result["chunks_created"]

    return {
        "users_processed": len(user_ids),
        "total_processed": total_processed,
        "total_chunks": total_chunks,
    }


@router.post("/admin/generate-weekly-reviews")
async def admin_generate_weekly_reviews() -> dict:
    """Generate weekly reviews for all active users."""
    from kensan_ai.batch.weekly_review import generate_weekly_reviews

    return await generate_weekly_reviews()


@router.post("/admin/run-prompt-optimization")
async def admin_run_prompt_optimization() -> dict:
    """Run prompt optimization batch for all active contexts."""
    from kensan_ai.batch.experiment_manager import run_prompt_optimization_batch

    return await run_prompt_optimization_batch()


# =============================================================================
# Explorer (Lakehouse Silver)
# =============================================================================

@router.get("/explorer/interactions")
async def get_explorer_interactions(
    authorization: str | None = Header(None),
    start_timestamp: str = Query(..., description="ISO8601 start time"),
    end_timestamp: str = Query(..., description="ISO8601 end time"),
) -> dict:
    """Get AI interactions from Lakehouse Silver layer."""
    user_id = _get_user_id_from_header(authorization)

    try:
        start = datetime.fromisoformat(start_timestamp.replace("Z", "+00:00"))
        end = datetime.fromisoformat(end_timestamp.replace("Z", "+00:00"))
    except ValueError as e:
        raise HTTPException(status_code=400, detail=f"Invalid timestamp format: {e}")

    reader = get_reader()
    interactions = reader.get_explorer_interactions(
        user_id=str(user_id),
        start=start,
        end=end,
    )

    return {"interactions": interactions}
