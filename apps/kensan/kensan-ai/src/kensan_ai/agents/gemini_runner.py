"""Gemini-based agent runner using Google GenAI SDK with Direct Tools."""

import asyncio
import json
import logging
import time
import uuid as uuid_module
from typing import AsyncGenerator, Any

from google import genai
from google.genai import types

from kensan_ai.config import get_settings
from kensan_ai.tools import get_tools_api_schema, execute_tool, format_tool_result, is_readonly_tool
from kensan_ai.agents.message_history import MessageHistory
from kensan_ai.agents.base import AgentResult, ToolCall
from kensan_ai.api.sse import sse_event, sse_keepalive
from kensan_ai.telemetry import get_tracer, get_genai_metrics

logger = logging.getLogger("kensan_ai.agent.gemini")

try:
    from opentelemetry import context as otel_context, trace
    from opentelemetry.trace import StatusCode

    _has_otel = True
except ImportError:
    _has_otel = False

_tracer = get_tracer("kensan-ai.agent.gemini")

_KEEPALIVE_INTERVAL = 10  # seconds


def _convert_tools_to_gemini(anthropic_tools: list[dict[str, Any]]) -> list[types.Tool]:
    """Convert Anthropic-format tool schemas to Gemini FunctionDeclaration format.

    Args:
        anthropic_tools: List of tool schemas in Anthropic format

    Returns:
        List of Gemini Tool objects with FunctionDeclarations
    """
    if not anthropic_tools:
        return []

    declarations = []
    for tool in anthropic_tools:
        schema = tool.get("input_schema", {})
        properties = schema.get("properties", {})
        required = schema.get("required", [])

        # Clean properties: remove unsupported keys for Gemini schema
        cleaned_properties: dict[str, Any] = {}
        for prop_name, prop_def in properties.items():
            cleaned = _clean_schema_for_gemini(prop_def)
            cleaned_properties[prop_name] = cleaned

        parameters: dict[str, Any] = {
            "type": "OBJECT",
            "properties": cleaned_properties,
        }
        if required:
            parameters["required"] = required

        declarations.append(types.FunctionDeclaration(
            name=tool["name"],
            description=tool.get("description", ""),
            parameters=parameters,
        ))

    return [types.Tool(function_declarations=declarations)]


def _clean_schema_for_gemini(schema: dict[str, Any]) -> dict[str, Any]:
    """Clean a JSON Schema property for Gemini compatibility.

    Gemini expects uppercase type names and doesn't support all JSON Schema features.
    """
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


def _build_gemini_contents(history: MessageHistory) -> list[types.Content]:
    """Convert Anthropic-style message history to Gemini Contents format.

    Gemini uses a different conversation format:
    - role: "user" or "model"
    - parts: list of Part objects (text, function_call, function_response)
    """
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
                # Tool results (Anthropic format) → function_response parts
                parts: list[types.Part] = []
                for item in content:
                    if item.get("type") == "tool_result":
                        result_content = item.get("content", "")
                        # Try to parse as JSON for structured response
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


class GeminiAgentRunner:
    """Agent runner using Google GenAI SDK with Direct Tools.

    Drop-in replacement for AgentRunner that uses Gemini instead of Claude.
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
        self.allowed_tools = allowed_tools
        self.max_turns = max_turns
        self.temperature = temperature
        self.model = model or get_settings().google_model
        self.context_id = context_id
        self.context_name = context_name
        self.context_version = context_version
        self.experiment_id = experiment_id
        self.deferred_tools = deferred_tools

        settings = get_settings()
        self.client = genai.Client(api_key=settings.google_api_key)

    def _unlock_deferred_tools(self) -> None:
        """Merge deferred_tools into allowed_tools (one-shot)."""
        if self.deferred_tools and self.allowed_tools is not None:
            self.allowed_tools = list(set(self.allowed_tools + self.deferred_tools))
            self.deferred_tools = None

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

    def _get_tools_schema(self) -> list[dict[str, Any]]:
        """Get the Anthropic-format tools schema (will be converted for Gemini)."""
        tools = get_tools_api_schema(self.allowed_tools)
        return tools

    async def run(self, prompt: str, user_id: str | None = None) -> AgentResult:
        """Run the agent with the given prompt (non-streaming).

        Args:
            prompt: The user's input prompt
            user_id: Optional user ID to inject into tool calls

        Returns:
            AgentResult with text response, tool calls, and token usage
        """
        all_tool_calls: list[ToolCall] = []
        total_input_tokens = 0
        total_output_tokens = 0
        final_text_parts: list[str] = []

        # Build conversation contents
        contents: list[types.Content] = [
            types.Content(role="user", parts=[types.Part.from_text(text=prompt)]),
        ]

        for turn in range(self.max_turns):
            # Recompute tools each turn (deferred unlock may change allowed_tools)
            anthropic_tools = self._get_tools_schema()
            gemini_tools = _convert_tools_to_gemini(anthropic_tools)

            response = await self.client.aio.models.generate_content(
                model=self.model,
                contents=contents,
                config=types.GenerateContentConfig(
                    system_instruction=self.system_prompt,
                    tools=gemini_tools if gemini_tools else None,
                    temperature=self.temperature,
                    max_output_tokens=4096,
                ),
            )

            # Track tokens
            if response.usage_metadata:
                total_input_tokens += response.usage_metadata.prompt_token_count or 0
                total_output_tokens += response.usage_metadata.candidates_token_count or 0

            # Process response parts
            function_calls: list[dict[str, Any]] = []
            model_parts: list[types.Part] = []

            if response.candidates and response.candidates[0].content:
                for part in response.candidates[0].content.parts:
                    if part.text:
                        final_text_parts.append(part.text)
                        model_parts.append(part)
                    elif part.function_call:
                        fc = part.function_call
                        function_calls.append({
                            "name": fc.name,
                            "args": dict(fc.args) if fc.args else {},
                        })
                        model_parts.append(part)

            # Add model response to contents
            contents.append(types.Content(role="model", parts=model_parts))

            if not function_calls:
                break

            # Execute tool calls
            response_parts: list[types.Part] = []
            for fc in function_calls:
                tool_name = fc["name"]
                tool_input = dict(fc["args"])

                if user_id and "user_id" not in tool_input:
                    tool_input["user_id"] = user_id

                try:
                    result = await execute_tool(tool_name, tool_input)
                    result_str = format_tool_result(result)
                    all_tool_calls.append(ToolCall(
                        id=f"tc_{uuid_module.uuid4().hex[:8]}",
                        name=tool_name,
                        input=tool_input,
                        output=result,
                    ))
                    try:
                        parsed_result = json.loads(result_str)
                    except (json.JSONDecodeError, TypeError):
                        parsed_result = {"result": result_str}
                    response_parts.append(types.Part.from_function_response(
                        name=tool_name,
                        response=parsed_result,
                    ))
                except Exception as e:
                    response_parts.append(types.Part.from_function_response(
                        name=tool_name,
                        response={"error": str(e)},
                    ))

            # Deferred unlock: if readonly tools were called, unlock write tools
            has_readonly = any(is_readonly_tool(fc["name"]) for fc in function_calls)
            if has_readonly and self.deferred_tools:
                self._unlock_deferred_tools()

            contents.append(types.Content(role="user", parts=response_parts))

        return AgentResult(
            text="\n".join(final_text_parts),
            tool_calls=all_tool_calls,
            tokens_input=total_input_tokens,
            tokens_output=total_output_tokens,
        )

    async def stream_sse(
        self,
        user_message: str,
        user_id: str | None = None,
        conversation_id: str | None = None,
        history: MessageHistory | None = None,
    ) -> AsyncGenerator[str, None]:
        """Run the agent with SSE event output.

        Readonly tools are executed immediately. Write tools are collected
        as pending actions and yielded as an action_proposal event.

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

        if _has_otel:
            stream_token = otel_context.attach(trace.set_span_in_context(stream_span))
        else:
            stream_token = None

        outcome = "success"
        actual_turns = 0

        prompt_sections = self._parse_prompt_sections(self.system_prompt)
        initial_tools = self._get_tools_schema()
        initial_tool_names = [t["name"] for t in initial_tools]

        logger.info(json.dumps({
            "event": "agent.prompt",
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
            "tool_count": len(initial_tools),
            "tool_names": initial_tool_names,
            "deferred_tools": self.deferred_tools or [],
            "tool_definitions_length": len(json.dumps(initial_tools, ensure_ascii=False)),
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
                for turn in range(self.max_turns):
                    actual_turns = turn + 1

                    turn_span = _tracer.start_span("gen_ai.turn")
                    turn_span.set_attribute("gen_ai.turn.number", turn + 1)
                    turn_span.set_attribute("gen_ai.request.model", self.model)

                    if _has_otel:
                        turn_token = otel_context.attach(trace.set_span_in_context(turn_span))
                    else:
                        turn_token = None

                    try:
                        # Recompute tools each turn (deferred unlock may change allowed_tools)
                        anthropic_tools = self._get_tools_schema()
                        gemini_tools = _convert_tools_to_gemini(anthropic_tools)

                        # Build Gemini contents from history
                        contents = _build_gemini_contents(history)

                        _start_keepalive()

                        # Use streaming for text output
                        assistant_content: list[dict[str, Any]] = []
                        function_calls: list[dict[str, Any]] = []
                        turn_input = 0
                        turn_output = 0

                        try:
                            response_stream = await self.client.aio.models.generate_content_stream(
                                model=self.model,
                                contents=contents,
                                config=types.GenerateContentConfig(
                                    system_instruction=self.system_prompt,
                                    tools=gemini_tools if gemini_tools else None,
                                    temperature=self.temperature,
                                    max_output_tokens=4096,
                                ),
                            )

                            first_chunk = True
                            async for chunk in response_stream:
                                if first_chunk:
                                    _stop_keepalive()
                                    first_chunk = False

                                if chunk.usage_metadata:
                                    turn_input = chunk.usage_metadata.prompt_token_count or 0
                                    turn_output = chunk.usage_metadata.candidates_token_count or 0

                                if not chunk.candidates:
                                    continue

                                for part in chunk.candidates[0].content.parts:
                                    if part.text:
                                        await event_queue.put(
                                            sse_event("text", {"content": part.text})
                                        )
                                        if assistant_content and assistant_content[-1].get("type") == "text":
                                            assistant_content[-1]["text"] += part.text
                                        else:
                                            assistant_content.append({"type": "text", "text": part.text})
                                    elif part.function_call:
                                        fc = part.function_call
                                        fc_id = f"tc_{uuid_module.uuid4().hex[:8]}"
                                        function_calls.append({
                                            "id": fc_id,
                                            "name": fc.name,
                                            "args": dict(fc.args) if fc.args else {},
                                        })
                                        assistant_content.append({
                                            "type": "tool_use",
                                            "id": fc_id,
                                            "name": fc.name,
                                            "input": dict(fc.args) if fc.args else {},
                                        })

                        except Exception as stream_err:
                            # Fallback to non-streaming if streaming fails
                            logger.warning(f"Streaming failed, falling back to non-streaming: {stream_err}")

                            response = await self.client.aio.models.generate_content(
                                model=self.model,
                                contents=contents,
                                config=types.GenerateContentConfig(
                                    system_instruction=self.system_prompt,
                                    tools=gemini_tools if gemini_tools else None,
                                    temperature=self.temperature,
                                    max_output_tokens=4096,
                                ),
                            )

                            if response.usage_metadata:
                                turn_input = response.usage_metadata.prompt_token_count or 0
                                turn_output = response.usage_metadata.candidates_token_count or 0

                            if response.candidates and response.candidates[0].content:
                                for part in response.candidates[0].content.parts:
                                    if part.text:
                                        await event_queue.put(
                                            sse_event("text", {"content": part.text})
                                        )
                                        assistant_content.append({"type": "text", "text": part.text})
                                    elif part.function_call:
                                        fc = part.function_call
                                        fc_id = f"tc_{uuid_module.uuid4().hex[:8]}"
                                        function_calls.append({
                                            "id": fc_id,
                                            "name": fc.name,
                                            "args": dict(fc.args) if fc.args else {},
                                        })
                                        assistant_content.append({
                                            "type": "tool_use",
                                            "id": fc_id,
                                            "name": fc.name,
                                            "input": dict(fc.args) if fc.args else {},
                                        })

                        _stop_keepalive()

                        total_input_tokens += turn_input
                        total_output_tokens += turn_output

                        turn_span.set_attribute("gen_ai.usage.input_tokens", turn_input)
                        turn_span.set_attribute("gen_ai.usage.output_tokens", turn_output)
                        turn_span.set_attribute("gen_ai.turn.tool_call_count", len(function_calls))

                        response_text = "".join(
                            b["text"] for b in assistant_content if b.get("type") == "text"
                        )
                        logger.info(json.dumps({
                            "event": "agent.turn",
                            "conversation_id": conv_id,
                            "turn_number": turn + 1,
                            "input_tokens": turn_input,
                            "output_tokens": turn_output,
                            "tool_call_count": len(function_calls),
                            "response_text": response_text,
                        }, ensure_ascii=False))

                        history.add_assistant_message(assistant_content)

                        if not function_calls:
                            break

                        # Process tool calls: readonly = execute, write = collect as proposals
                        tool_results: list[dict[str, Any]] = []
                        has_readonly_tools = False

                        _start_keepalive()

                        readonly_calls: list[dict[str, Any]] = []
                        write_calls: list[dict[str, Any]] = []
                        for fc in function_calls:
                            tool_input = dict(fc["args"])
                            if user_id and "user_id" not in tool_input:
                                tool_input["user_id"] = user_id
                            prepared = {**fc, "input": tool_input}
                            if is_readonly_tool(fc["name"]):
                                readonly_calls.append(prepared)
                            else:
                                write_calls.append(prepared)

                        # Execute readonly tools in parallel
                        if readonly_calls:
                            has_readonly_tools = True

                            for tc in readonly_calls:
                                await event_queue.put(sse_event("tool_call", {
                                    "id": tc["id"],
                                    "name": tc["name"],
                                }))

                            async def _exec_readonly(tc: dict[str, Any]) -> dict[str, Any]:
                                tool_span = _tracer.start_span("agent.tool_execution")
                                tool_span.set_attribute("tool.name", tc["name"])
                                tool_span.set_attribute("tool.readonly", True)
                                try:
                                    result = await execute_tool(tc["name"], tc["input"])
                                    result_str = format_tool_result(result)
                                    tool_span.set_attribute("tool.success", True)
                                    logger.info(json.dumps({
                                        "event": "agent.tool_call",
                                        "conversation_id": conv_id,
                                        "tool_name": tc["name"],
                                        "tool_input": json.dumps(tc["input"], ensure_ascii=False),
                                        "tool_output": result_str,
                                        "success": True,
                                    }, ensure_ascii=False))
                                    return {
                                        "type": "tool_result",
                                        "tool_use_id": tc["id"],
                                        "_tool_name": tc["name"],
                                        "content": result_str,
                                    }
                                except Exception as e:
                                    tool_span.set_attribute("tool.success", False)
                                    tool_span.set_attribute("tool.error", str(e))
                                    if _has_otel:
                                        tool_span.set_status(StatusCode.ERROR, str(e))
                                        tool_span.record_exception(e)
                                    logger.warning(json.dumps({
                                        "event": "agent.tool_call",
                                        "conversation_id": conv_id,
                                        "tool_name": tc["name"],
                                        "tool_input": json.dumps(tc["input"], ensure_ascii=False),
                                        "success": False,
                                        "error": str(e),
                                    }, ensure_ascii=False))
                                    return {
                                        "type": "tool_result",
                                        "tool_use_id": tc["id"],
                                        "_tool_name": tc["name"],
                                        "content": json.dumps({"error": str(e)}, ensure_ascii=False),
                                        "is_error": True,
                                    }
                                finally:
                                    tool_span.end()

                            parallel_results = await asyncio.gather(
                                *[_exec_readonly(tc) for tc in readonly_calls]
                            )
                            tool_results.extend(parallel_results)

                            for tc in readonly_calls:
                                await event_queue.put(sse_event("tool_result", {
                                    "id": tc["id"],
                                    "name": tc["name"],
                                }))

                        # Collect write tools as pending actions
                        for tc in write_calls:
                            action_id = str(uuid_module.uuid4())
                            pending_actions.append({
                                "id": action_id,
                                "tool_name": tc["name"],
                                "description": f"{tc['name']}({json.dumps(tc['input'], ensure_ascii=False)})",
                                "input": tc["input"],
                            })
                            tool_results.append({
                                "type": "tool_result",
                                "tool_use_id": tc["id"],
                                "_tool_name": tc["name"],
                                "content": f"この操作はユーザーの承認待ちです: {tc['name']}",
                            })

                        _stop_keepalive()

                        # Deferred unlock: readonly tool execution signals write intent
                        if has_readonly_tools and self.deferred_tools:
                            self._unlock_deferred_tools()

                        if tool_results:
                            history.add_tool_results(tool_results)

                        if not has_readonly_tools:
                            break

                    finally:
                        _stop_keepalive()
                        if _has_otel and turn_token is not None:
                            otel_context.detach(turn_token)
                        turn_span.end()
                else:
                    outcome = "max_turns_reached"

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
            }
            token_counter.add(total_input_tokens, {**_metric_attrs, "gen_ai.token.type": "input"})
            token_counter.add(total_output_tokens, {**_metric_attrs, "gen_ai.token.type": "output"})
            duration_hist.record(_duration, _metric_attrs)
            op_counter.add(1, _metric_attrs)

            logger.info(json.dumps({
                "event": "agent.complete",
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
            "tokens": {
                "input": total_input_tokens,
                "output": total_output_tokens,
            },
        })
