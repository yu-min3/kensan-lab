"""Google ADK-based agent runner using Agent Development Kit."""

import asyncio
import json
import logging
import time
import uuid as uuid_module
from typing import AsyncGenerator, Any

from google.adk.agents import Agent
from google.adk.runners import Runner
from google.adk.sessions import InMemorySessionService
from google.adk.tools import BaseTool
from google.genai import types

from kensan_ai.config import get_settings
from kensan_ai.tools import get_tools_api_schema, execute_tool, format_tool_result, is_readonly_tool
from kensan_ai.tools.base import get_tool, get_all_tools, ToolDefinition
from kensan_ai.agents.message_history import MessageHistory
from kensan_ai.agents.base import AgentResult, ToolCall
from kensan_ai.api.sse import sse_event, sse_keepalive
from kensan_ai.telemetry import get_tracer, get_genai_metrics

logger = logging.getLogger("kensan_ai.agent.adk")

try:
    from opentelemetry import context as otel_context, trace
    from opentelemetry.trace import StatusCode

    _has_otel = True
except ImportError:
    _has_otel = False

_tracer = get_tracer("kensan-ai.agent.adk")

_KEEPALIVE_INTERVAL = 10  # seconds


def _clean_schema_for_gemini(schema: dict[str, Any]) -> dict[str, Any]:
    """Clean a JSON Schema property for Gemini compatibility."""
    result: dict[str, Any] = {}

    if "type" in schema:
        type_map = {
            "string": "STRING",
            "integer": "INTEGER",
            "number": "NUMBER",
            "boolean": "BOOLEAN",
            "array": "ARRAY",
            "object": "OBJECT",
        }
        result["type"] = type_map.get(schema["type"], schema["type"])

    if "description" in schema:
        result["description"] = schema["description"]

    if "enum" in schema:
        result["enum"] = schema["enum"]

    if "items" in schema:
        result["items"] = _clean_schema_for_gemini(schema["items"])

    if "properties" in schema:
        result["properties"] = {
            k: _clean_schema_for_gemini(v) for k, v in schema["properties"].items()
        }

    if "required" in schema:
        result["required"] = schema["required"]

    return result


class KensanTool(BaseTool):
    """ADK BaseTool wrapper around a ToolDefinition.

    For readonly tools, executes the handler immediately.
    For write tools, collects the call into pending_actions for user approval.
    """

    def __init__(
        self,
        tool_def: ToolDefinition,
        user_id: str,
        pending_actions: list[dict[str, Any]],
    ):
        self._tool_def = tool_def
        self._user_id = user_id
        self._pending_actions = pending_actions

    @property
    def name(self) -> str:
        return self._tool_def.name

    @property
    def description(self) -> str:
        return self._tool_def.description

    def _get_declaration(self) -> types.FunctionDeclaration:
        """Convert ToolDefinition schema to Gemini FunctionDeclaration."""
        schema = self._tool_def.input_schema
        properties = schema.get("properties", {})
        required = schema.get("required", [])

        cleaned_properties: dict[str, Any] = {}
        for prop_name, prop_def in properties.items():
            cleaned_properties[prop_name] = _clean_schema_for_gemini(prop_def)

        parameters: dict[str, Any] = {
            "type": "OBJECT",
            "properties": cleaned_properties,
        }
        if required:
            parameters["required"] = required

        return types.FunctionDeclaration(
            name=self._tool_def.name,
            description=self._tool_def.description,
            parameters=parameters,
        )

    async def run_async(self, *, args: dict[str, Any], tool_context: Any) -> Any:
        """Execute the tool (readonly) or collect as pending action (write)."""
        args["user_id"] = self._user_id

        if self._tool_def.readonly:
            result = await self._tool_def.handler(args)
            return result
        else:
            action_id = str(uuid_module.uuid4())
            self._pending_actions.append({
                "id": action_id,
                "tool_name": self._tool_def.name,
                "description": f"{self._tool_def.name}({json.dumps(args, ensure_ascii=False)})",
                "input": args,
            })
            return {"status": "pending", "message": f"この操作はユーザーの承認待ちです: {self._tool_def.name}"}


def _build_gemini_contents(history: MessageHistory) -> list[types.Content]:
    """Convert Anthropic-style message history to Gemini Contents format."""
    contents: list[types.Content] = []

    for msg in history.get_messages():
        role = msg["role"]
        content = msg["content"]

        if role == "user":
            if isinstance(content, str):
                contents.append(types.Content(
                    role="user",
                    parts=[types.Part.from_text(text=content)],
                ))
            elif isinstance(content, list):
                parts: list[types.Part] = []
                for item in content:
                    if item.get("type") == "tool_result":
                        result_content = item.get("content", "")
                        try:
                            parsed = json.loads(result_content)
                        except (json.JSONDecodeError, TypeError):
                            parsed = {"result": result_content}

                        parts.append(types.Part.from_function_response(
                            name=item.get("_tool_name", "unknown"),
                            response=parsed,
                        ))
                    else:
                        parts.append(types.Part.from_text(text=str(item)))
                if parts:
                    contents.append(types.Content(role="user", parts=parts))

        elif role == "assistant":
            parts = []
            if isinstance(content, list):
                for block in content:
                    if block.get("type") == "text":
                        parts.append(types.Part.from_text(text=block["text"]))
                    elif block.get("type") == "tool_use":
                        parts.append(types.Part.from_function_call(
                            name=block["name"],
                            args=block.get("input", {}),
                        ))
            if parts:
                contents.append(types.Content(role="model", parts=parts))

    return contents


class AdkAgentRunner:
    """Agent runner using Google ADK (Agent Development Kit).

    Drop-in replacement for GeminiAgentRunner that uses ADK Agent + Runner.
    Maintains the same interface for stream_sse().
    """

    def __init__(
        self,
        system_prompt: str,
        allowed_tools: list[str] | None = None,
        max_turns: int = 10,
        temperature: float = 0.7,
        model: str | None = None,
        context_id: str | None = None,
        context_name: str | None = None,
        context_version: str | None = None,
        experiment_id: str | None = None,
        deferred_tools: list[str] | None = None,
    ):
        self.system_prompt = system_prompt
        # ADK manages its own loop internally, so mid-loop injection is not possible.
        # Merge deferred_tools into allowed_tools upfront.
        if deferred_tools and allowed_tools is not None:
            self.allowed_tools = list(set(allowed_tools + deferred_tools))
        else:
            self.allowed_tools = allowed_tools
        self.max_turns = max_turns
        self.temperature = temperature
        self.model = model or get_settings().google_model
        self.context_id = context_id
        self.context_name = context_name
        self.context_version = context_version
        self.experiment_id = experiment_id

    @staticmethod
    def _parse_prompt_sections(prompt: str) -> dict[str, int]:
        """Parse system prompt into sections by ## headers and return char counts."""
        import re
        sections: dict[str, int] = {}
        parts = re.split(r'^(## .+)$', prompt, flags=re.MULTILINE)
        if parts[0].strip():
            sections["(ベース指示)"] = len(parts[0].strip())
        for i in range(1, len(parts), 2):
            header = parts[i].replace("## ", "").strip()
            content = parts[i + 1] if i + 1 < len(parts) else ""
            sections[header] = len(parts[i]) + len(content)
        return sections

    def _get_tool_definitions(self) -> list[ToolDefinition]:
        """Get ToolDefinition objects for allowed tools."""
        all_tools = get_all_tools()
        if self.allowed_tools is None:
            return all_tools
        return [t for t in all_tools if t.name in self.allowed_tools]

    def _get_tools_schema(self) -> list[dict[str, Any]]:
        """Get the Anthropic-format tools schema."""
        return get_tools_api_schema(self.allowed_tools)

    async def run(self, prompt: str, user_id: str | None = None) -> AgentResult:
        """Run the agent with the given prompt (non-streaming).

        Args:
            prompt: The user's input prompt
            user_id: Optional user ID to inject into tool calls

        Returns:
            AgentResult with text response, tool calls, and token usage
        """
        pending_actions: list[dict[str, Any]] = []
        tool_defs = self._get_tool_definitions()

        kensan_tools = [
            KensanTool(td, user_id or "", pending_actions)
            for td in tool_defs
        ]

        agent = Agent(
            name="kensan-chat",
            model=self.model,
            instruction=self.system_prompt,
            tools=kensan_tools,
        )

        session_service = InMemorySessionService()
        session = await session_service.create_session(
            app_name="kensan-ai",
            user_id=user_id or "anonymous",
        )

        runner = Runner(
            agent=agent,
            app_name="kensan-ai",
            session_service=session_service,
        )

        from google.genai.types import Content, Part

        user_content = Content(
            role="user",
            parts=[Part.from_text(text=prompt)],
        )

        all_text_parts: list[str] = []
        all_tool_calls: list[ToolCall] = []

        async for event in runner.run_async(
            user_id=session.user_id,
            session_id=session.id,
            new_message=user_content,
        ):
            if event.content and event.content.parts:
                for part in event.content.parts:
                    if part.text:
                        all_text_parts.append(part.text)
                    elif part.function_call:
                        fc = part.function_call
                        all_tool_calls.append(ToolCall(
                            id=f"tc_{uuid_module.uuid4().hex[:8]}",
                            name=fc.name,
                            input=dict(fc.args) if fc.args else {},
                            output=None,
                        ))

        return AgentResult(
            text="".join(all_text_parts),
            tool_calls=all_tool_calls,
            tokens_input=0,
            tokens_output=0,
        )

    async def stream_sse(
        self,
        user_message: str,
        user_id: str | None = None,
        conversation_id: str | None = None,
        history: MessageHistory | None = None,
    ) -> AsyncGenerator[str, None]:
        """Run the agent with SSE event output.

        Readonly tools are executed by ADK automatically via KensanTool.
        Write tools are collected as pending actions and yielded as an action_proposal event.

        Args:
            user_message: The user's input message
            user_id: Optional user ID to inject into tool calls
            conversation_id: Conversation ID for state tracking
            history: Optional existing message history to continue

        Yields:
            SSE event strings
        """
        if history is None:
            history = MessageHistory()
        history.add_user_message(user_message)

        anthropic_tools = self._get_tools_schema()
        tool_defs = self._get_tool_definitions()

        conv_id = conversation_id or str(uuid_module.uuid4())
        pending_actions: list[dict[str, Any]] = []
        total_input_tokens = 0
        total_output_tokens = 0

        # Queue for interleaving keepalive events with normal SSE events
        event_queue: asyncio.Queue[str | None] = asyncio.Queue()
        keepalive_task: asyncio.Task[None] | None = None

        async def _keepalive_loop() -> None:
            try:
                while True:
                    await asyncio.sleep(_KEEPALIVE_INTERVAL)
                    await event_queue.put(sse_keepalive())
            except asyncio.CancelledError:
                pass

        def _start_keepalive() -> None:
            nonlocal keepalive_task
            if keepalive_task is None or keepalive_task.done():
                keepalive_task = asyncio.create_task(_keepalive_loop())

        def _stop_keepalive() -> None:
            nonlocal keepalive_task
            if keepalive_task is not None and not keepalive_task.done():
                keepalive_task.cancel()
                keepalive_task = None

        _start_time = time.monotonic()

        # Start root span
        stream_span = _tracer.start_span("agent.stream")
        stream_span.set_attribute("gen_ai.user.id", user_id or "")
        stream_span.set_attribute("gen_ai.conversation.id", conv_id)
        stream_span.set_attribute("gen_ai.prompt", user_message[:500])
        stream_span.set_attribute("gen_ai.request.model", self.model)
        stream_span.set_attribute("gen_ai.request.max_turns", self.max_turns)
        stream_span.set_attribute("gen_ai.runner", "adk")

        if _has_otel:
            stream_token = otel_context.attach(trace.set_span_in_context(stream_span))
        else:
            stream_token = None

        outcome = "success"
        actual_turns = 0

        prompt_sections = self._parse_prompt_sections(self.system_prompt)
        tool_names = [t["name"] for t in anthropic_tools]

        logger.info(json.dumps({
            "event": "agent.prompt",
            "runner": "adk",
            "user_id": user_id or "",
            "conversation_id": conv_id,
            "user_message": user_message,
            "model": self.model,
            "context_id": self.context_id or "",
            "context_name": self.context_name or "",
            "context_version": self.context_version or "",
            "experiment_id": self.experiment_id or "",
            "system_prompt_length": len(self.system_prompt),
            "system_prompt_sections": prompt_sections,
            "tool_count": len(anthropic_tools),
            "tool_names": tool_names,
            "tool_definitions_length": len(json.dumps(anthropic_tools, ensure_ascii=False)),
        }, ensure_ascii=False))

        logger.info(json.dumps({
            "event": "agent.system_prompt",
            "conversation_id": conv_id,
            "context_id": self.context_id or "",
            "context_name": self.context_name or "",
            "context_version": self.context_version or "",
            "system_prompt": self.system_prompt,
        }, ensure_ascii=False))

        async def _run_agent() -> None:
            nonlocal total_input_tokens, total_output_tokens, actual_turns, outcome

            try:
                # Build KensanTools with shared pending_actions
                kensan_tools = [
                    KensanTool(td, user_id or "", pending_actions)
                    for td in tool_defs
                ]

                agent = Agent(
                    name="kensan-chat",
                    model=self.model,
                    instruction=self.system_prompt,
                    tools=kensan_tools,
                )

                session_service = InMemorySessionService()
                session = await session_service.create_session(
                    app_name="kensan-ai",
                    user_id=user_id or "anonymous",
                )

                # Inject conversation history into session state
                history_contents = _build_gemini_contents(history)
                # The last message in history is the current user_message (just added above).
                # We feed prior messages as session history, and the last user message as new_message.
                prior_contents = history_contents[:-1] if len(history_contents) > 1 else []
                current_user_content = history_contents[-1] if history_contents else types.Content(
                    role="user",
                    parts=[types.Part.from_text(text=user_message)],
                )

                # Inject prior conversation history into the session
                if prior_contents:
                    session_state = await session_service.get_session(
                        app_name="kensan-ai",
                        user_id=session.user_id,
                        session_id=session.id,
                    )
                    if session_state:
                        # ADK stores conversation in session events; we inject via
                        # appending to session directly
                        for content in prior_contents:
                            session_state.events.append(
                                _content_to_event(content)
                            )

                runner = Runner(
                    agent=agent,
                    app_name="kensan-ai",
                    session_service=session_service,
                )

                _start_keepalive()

                turn_number = 0
                assistant_content: list[dict[str, Any]] = []

                async for event in runner.run_async(
                    user_id=session.user_id,
                    session_id=session.id,
                    new_message=current_user_content,
                ):
                    _stop_keepalive()

                    if not event.content or not event.content.parts:
                        _start_keepalive()
                        continue

                    turn_number += 1
                    actual_turns = turn_number

                    for part in event.content.parts:
                        if part.text:
                            await event_queue.put(
                                sse_event("text", {"content": part.text})
                            )
                            # Track for history
                            if assistant_content and assistant_content[-1].get("type") == "text":
                                assistant_content[-1]["text"] += part.text
                            else:
                                assistant_content.append({"type": "text", "text": part.text})

                        elif part.function_call:
                            fc = part.function_call
                            fc_id = f"tc_{uuid_module.uuid4().hex[:8]}"
                            tool_name = fc.name

                            if is_readonly_tool(tool_name):
                                await event_queue.put(sse_event("tool_call", {
                                    "id": fc_id,
                                    "name": tool_name,
                                }))

                            assistant_content.append({
                                "type": "tool_use",
                                "id": fc_id,
                                "name": tool_name,
                                "input": dict(fc.args) if fc.args else {},
                            })

                        elif part.function_response:
                            fr = part.function_response
                            tool_name = fr.name

                            if is_readonly_tool(tool_name):
                                await event_queue.put(sse_event("tool_result", {
                                    "id": f"tr_{uuid_module.uuid4().hex[:8]}",
                                    "name": tool_name,
                                }))

                                logger.info(json.dumps({
                                    "event": "agent.tool_call",
                                    "conversation_id": conv_id,
                                    "tool_name": tool_name,
                                    "success": True,
                                }, ensure_ascii=False))

                    _start_keepalive()

                _stop_keepalive()

                # Track token usage from session if available
                # ADK doesn't directly expose token counts in all cases,
                # so we extract from the session's last event if available
                try:
                    final_session = await session_service.get_session(
                        app_name="kensan-ai",
                        user_id=session.user_id,
                        session_id=session.id,
                    )
                    if final_session and final_session.events:
                        for evt in final_session.events:
                            if hasattr(evt, "usage_metadata") and evt.usage_metadata:
                                total_input_tokens += getattr(evt.usage_metadata, "prompt_token_count", 0) or 0
                                total_output_tokens += getattr(evt.usage_metadata, "candidates_token_count", 0) or 0
                except Exception:
                    pass

                response_text = "".join(
                    b["text"] for b in assistant_content if b.get("type") == "text"
                )
                logger.info(json.dumps({
                    "event": "agent.turn",
                    "conversation_id": conv_id,
                    "turn_number": actual_turns,
                    "input_tokens": total_input_tokens,
                    "output_tokens": total_output_tokens,
                    "response_text": response_text,
                }, ensure_ascii=False))

                # Update message history
                if assistant_content:
                    history.add_assistant_message(assistant_content)

                # If there were write tool calls, add their "pending" responses to history
                if pending_actions:
                    tool_results = []
                    for action in pending_actions:
                        tool_results.append({
                            "type": "tool_result",
                            "tool_use_id": f"pa_{action['id'][:8]}",
                            "_tool_name": action["tool_name"],
                            "content": f"この操作はユーザーの承認待ちです: {action['tool_name']}",
                        })
                    if tool_results:
                        history.add_tool_results(tool_results)

            except Exception as exc:
                outcome = "error"
                if _has_otel:
                    stream_span.set_status(StatusCode.ERROR, str(exc))
                    stream_span.record_exception(exc)
                raise
            finally:
                _stop_keepalive()

        # Run the agent logic as a task so we can yield events from the queue
        agent_task = asyncio.create_task(_run_agent())
        agent_error: Exception | None = None

        try:
            while True:
                if agent_task.done():
                    while not event_queue.empty():
                        event_str = event_queue.get_nowait()
                        if event_str is not None:
                            yield event_str
                    agent_error = agent_task.exception()
                    break

                try:
                    event_str = await asyncio.wait_for(event_queue.get(), timeout=0.1)
                    if event_str is not None:
                        yield event_str
                except asyncio.TimeoutError:
                    continue
        finally:
            _stop_keepalive()

            stream_span.set_attribute("gen_ai.usage.total_turns", actual_turns)
            stream_span.set_attribute("gen_ai.usage.input_tokens", total_input_tokens)
            stream_span.set_attribute("gen_ai.usage.output_tokens", total_output_tokens)
            stream_span.set_attribute("gen_ai.response.pending_action_count", len(pending_actions))
            stream_span.set_attribute("gen_ai.response.outcome", outcome)

            _duration = time.monotonic() - _start_time
            token_counter, duration_hist, op_counter = get_genai_metrics()
            _metric_attrs = {
                "gen_ai.request.model": self.model,
                "gen_ai.response.outcome": outcome,
                "gen_ai.runner": "adk",
            }
            token_counter.add(total_input_tokens, {**_metric_attrs, "gen_ai.token.type": "input"})
            token_counter.add(total_output_tokens, {**_metric_attrs, "gen_ai.token.type": "output"})
            duration_hist.record(_duration, _metric_attrs)
            op_counter.add(1, _metric_attrs)

            logger.info(json.dumps({
                "event": "agent.complete",
                "runner": "adk",
                "conversation_id": conv_id,
                "outcome": outcome,
                "total_turns": actual_turns,
                "total_input_tokens": total_input_tokens,
                "total_output_tokens": total_output_tokens,
                "pending_action_count": len(pending_actions),
            }, ensure_ascii=False))

            if _has_otel and stream_token is not None:
                otel_context.detach(stream_token)
            stream_span.end()

        if agent_error is not None:
            raise agent_error

        if pending_actions:
            yield sse_event("action_proposal", {"actions": pending_actions})

        yield sse_event("done", {
            "conversation_id": conv_id,
            "model": self.model,
            "tokens": {
                "input": total_input_tokens,
                "output": total_output_tokens,
            },
        })


def _content_to_event(content: types.Content) -> Any:
    """Convert a Gemini Content object to an ADK session Event for history injection."""
    from google.adk.events import Event

    return Event(
        id=str(uuid_module.uuid4()),
        author=content.role if content.role == "user" else "kensan-chat",
        content=content,
    )
