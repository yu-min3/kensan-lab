"""Base agent runner using Direct Tools with Anthropic API."""

import asyncio
import json
import logging
import time
import uuid as uuid_module
from dataclasses import dataclass, field
from typing import AsyncGenerator, AsyncIterator, Any

import anthropic

from kensan_ai.config import get_settings
from kensan_ai.tools import get_tools_api_schema, execute_tool, format_tool_result, is_readonly_tool
from kensan_ai.agents.message_history import MessageHistory
from kensan_ai.api.sse import sse_event, sse_keepalive
from kensan_ai.telemetry import get_tracer, get_genai_metrics

logger = logging.getLogger("kensan_ai.agent")

try:
    from opentelemetry import context as otel_context, trace
    from opentelemetry.trace import StatusCode

    _has_otel = True
except ImportError:
    _has_otel = False

_tracer = get_tracer("kensan-ai.agent")

_KEEPALIVE_INTERVAL = 10  # seconds


@dataclass
class ToolCall:
    """Record of a tool call made during agent execution."""

    id: str
    name: str
    input: dict[str, Any]
    output: Any


@dataclass
class AgentResult:
    """Result of an agent execution."""

    text: str
    tool_calls: list[ToolCall] = field(default_factory=list)
    tokens_input: int = 0
    tokens_output: int = 0


class AgentRunner:
    """Base class for running agents with Direct Tools."""

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
        """Initialize the agent runner.

        Args:
            system_prompt: The system prompt for the agent
            allowed_tools: List of tool names to allow. If None, all tools are allowed.
            max_turns: Maximum number of agent turns (tool call cycles)
            temperature: Temperature for the model
            model: Model to use. If None, uses default from settings.
            context_id: AI context ID from database
            context_name: AI context name for identification
            context_version: AI context version string
            experiment_id: A/B test experiment ID
            deferred_tools: Write tools to unlock after first readonly tool execution.
                           These are added to allowed_tools dynamically when the LLM
                           calls a readonly tool, signaling write intent.
        """
        self.system_prompt = system_prompt
        self.allowed_tools = allowed_tools
        self.max_turns = max_turns
        self.temperature = temperature
        self.model = model or get_settings().anthropic_model
        self.context_id = context_id
        self.context_name = context_name
        self.context_version = context_version
        self.experiment_id = experiment_id
        self.deferred_tools = deferred_tools

        # Initialize client
        settings = get_settings()
        self.client = anthropic.AsyncAnthropic(api_key=settings.anthropic_api_key)

    @staticmethod
    async def _execute_tool_calls(
        tool_use_blocks: list[dict[str, Any]],
        user_id: str | None,
    ) -> tuple[list[dict[str, Any]], list[ToolCall]]:
        """Execute tool calls and return (tool_results, tool_call_records).

        Injects user_id into tool inputs when provided.
        """
        tool_results: list[dict[str, Any]] = []
        tool_call_records: list[ToolCall] = []
        for tool_use in tool_use_blocks:
            tool_name = tool_use["name"]
            tool_input = dict(tool_use["input"])

            if user_id and "user_id" not in tool_input:
                tool_input["user_id"] = user_id

            try:
                result = await execute_tool(tool_name, tool_input)
                result_str = format_tool_result(result)

                tool_call_records.append(ToolCall(
                    id=tool_use["id"],
                    name=tool_name,
                    input=tool_input,
                    output=result,
                ))

                tool_results.append({
                    "type": "tool_result",
                    "tool_use_id": tool_use["id"],
                    "content": result_str,
                })
            except Exception as e:
                tool_results.append({
                    "type": "tool_result",
                    "tool_use_id": tool_use["id"],
                    "content": json.dumps({"error": str(e)}, ensure_ascii=False),
                    "is_error": True,
                })

        return tool_results, tool_call_records

    @staticmethod
    def _parse_prompt_sections(prompt: str) -> dict[str, int]:
        """Parse system prompt into sections by ## headers and return char counts."""
        import re
        sections: dict[str, int] = {}
        # Split by ## headers
        parts = re.split(r'^(## .+)$', prompt, flags=re.MULTILINE)
        # parts[0] is text before first ##, parts[1] is first header, parts[2] is content, etc.
        if parts[0].strip():
            sections["(ベース指示)"] = len(parts[0].strip())
        for i in range(1, len(parts), 2):
            header = parts[i].replace("## ", "").strip()
            content = parts[i + 1] if i + 1 < len(parts) else ""
            sections[header] = len(parts[i]) + len(content)
        return sections

    def _get_tools_schema(self) -> list[dict[str, Any]]:
        """Get the tools schema for the API call.

        Adds cache_control to the last tool for prompt caching.
        """
        tools = get_tools_api_schema(self.allowed_tools)
        if tools:
            tools[-1]["cache_control"] = {"type": "ephemeral"}
        return tools

    def _get_system_blocks(self) -> list[dict[str, Any]]:
        """Get system prompt as content blocks with cache_control for prompt caching."""
        return [
            {
                "type": "text",
                "text": self.system_prompt,
                "cache_control": {"type": "ephemeral"},
            }
        ]

    def _unlock_deferred_tools(self) -> None:
        """Merge deferred_tools into allowed_tools (one-shot)."""
        if self.deferred_tools and self.allowed_tools is not None:
            self.allowed_tools = list(set(self.allowed_tools + self.deferred_tools))
            self.deferred_tools = None

    async def run(self, prompt: str, user_id: str | None = None) -> AgentResult:
        """Run the agent with the given prompt and return the response.

        Args:
            prompt: The user's input prompt
            user_id: Optional user ID to inject into tool calls

        Returns:
            AgentResult with text response, tool calls, and token usage
        """
        history = MessageHistory()
        history.add_user_message(prompt)

        all_tool_calls: list[ToolCall] = []
        total_input_tokens = 0
        total_output_tokens = 0
        final_text_parts: list[str] = []

        for turn in range(self.max_turns):
            # Recompute tools each turn (deferred unlock may change allowed_tools)
            tools = self._get_tools_schema()

            # Make API call
            response = await self.client.messages.create(
                model=self.model,
                max_tokens=4096,
                system=self._get_system_blocks(),
                messages=history.get_messages(),
                tools=tools if tools else anthropic.NOT_GIVEN,
                temperature=self.temperature,
            )

            # Track token usage
            total_input_tokens += response.usage.input_tokens
            total_output_tokens += response.usage.output_tokens

            # Process response
            assistant_content: list[dict[str, Any]] = []
            tool_use_blocks: list[dict[str, Any]] = []

            for block in response.content:
                if block.type == "text":
                    final_text_parts.append(block.text)
                    assistant_content.append({"type": "text", "text": block.text})
                elif block.type == "tool_use":
                    tool_use_blocks.append({
                        "type": "tool_use",
                        "id": block.id,
                        "name": block.name,
                        "input": block.input,
                    })
                    assistant_content.append({
                        "type": "tool_use",
                        "id": block.id,
                        "name": block.name,
                        "input": block.input,
                    })

            # Add assistant message
            history.add_assistant_message(assistant_content)

            # If no tool calls, we're done
            if not tool_use_blocks:
                break

            # Execute tool calls
            tool_results, new_tool_calls = await self._execute_tool_calls(
                tool_use_blocks, user_id
            )
            all_tool_calls.extend(new_tool_calls)

            # Deferred unlock: if readonly tools were called, unlock write tools
            has_readonly = any(is_readonly_tool(tc["name"]) for tc in tool_use_blocks)
            if has_readonly and self.deferred_tools:
                self._unlock_deferred_tools()

            # Add tool results as user message
            history.add_tool_results(tool_results)

            # Check if we should stop (end_turn)
            if response.stop_reason == "end_turn":
                break

        return AgentResult(
            text="\n".join(final_text_parts),
            tool_calls=all_tool_calls,
            tokens_input=total_input_tokens,
            tokens_output=total_output_tokens,
        )

    async def stream(self, prompt: str, user_id: str | None = None) -> AsyncIterator[str]:
        """Run the agent with streaming response.

        This simplified streaming implementation yields text as it arrives,
        but still processes tool calls synchronously between streams.

        Args:
            prompt: The user's input prompt
            user_id: Optional user ID to inject into tool calls

        Yields:
            Text chunks as they arrive from the model
        """
        history = MessageHistory()
        history.add_user_message(prompt)

        for turn in range(self.max_turns):
            # Recompute tools each turn (deferred unlock may change allowed_tools)
            tools = self._get_tools_schema()

            # Collect response using streaming
            assistant_content: list[dict[str, Any]] = []
            tool_use_blocks: list[dict[str, Any]] = []
            current_tool_use: dict[str, Any] | None = None
            current_tool_input_json = ""

            async with self.client.messages.stream(
                model=self.model,
                max_tokens=4096,
                system=self._get_system_blocks(),
                messages=history.get_messages(),
                tools=tools if tools else anthropic.NOT_GIVEN,
                temperature=self.temperature,
            ) as stream:
                async for event in stream:
                    if event.type == "content_block_start":
                        if event.content_block.type == "text":
                            pass  # Will handle in delta
                        elif event.content_block.type == "tool_use":
                            current_tool_use = {
                                "type": "tool_use",
                                "id": event.content_block.id,
                                "name": event.content_block.name,
                                "input": {},
                            }
                            current_tool_input_json = ""
                    elif event.type == "content_block_delta":
                        if event.delta.type == "text_delta":
                            yield event.delta.text
                            # Also track for message history
                            if assistant_content and assistant_content[-1].get("type") == "text":
                                assistant_content[-1]["text"] += event.delta.text
                            else:
                                assistant_content.append({"type": "text", "text": event.delta.text})
                        elif event.delta.type == "input_json_delta":
                            current_tool_input_json += event.delta.partial_json
                    elif event.type == "content_block_stop":
                        if current_tool_use:
                            # Parse accumulated JSON input
                            if current_tool_input_json:
                                try:
                                    current_tool_use["input"] = json.loads(current_tool_input_json)
                                except json.JSONDecodeError:
                                    current_tool_use["input"] = {}
                            tool_use_blocks.append(current_tool_use)
                            assistant_content.append(current_tool_use)
                            current_tool_use = None
                            current_tool_input_json = ""

                # Get final message for stop reason
                final_message = await stream.get_final_message()

            # Add assistant message to history
            history.add_assistant_message(assistant_content)

            # If no tool calls, we're done
            if not tool_use_blocks:
                break

            # Execute tool calls (not streamed)
            tool_results, _ = await self._execute_tool_calls(
                tool_use_blocks, user_id
            )

            # Deferred unlock: if readonly tools were called, unlock write tools
            has_readonly = any(is_readonly_tool(tc["name"]) for tc in tool_use_blocks)
            if has_readonly and self.deferred_tools:
                self._unlock_deferred_tools()

            history.add_tool_results(tool_results)

            if final_message.stop_reason == "end_turn":
                break

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

        Sends periodic keepalive SSE events during tool execution and
        Claude API wait to prevent client-side timeouts.

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
            """Send keepalive events at regular intervals."""
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

        # Start timer for duration metric
        _start_time = time.monotonic()

        # Start root span (manual lifecycle for async generator)
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
        tool_nudged = False   # Track if we've nudged to call any tools (turn 0)
        write_nudged = False  # Track if we've nudged for write tools (turn 1+)

        prompt_sections = self._parse_prompt_sections(self.system_prompt)
        initial_tools = self._get_tools_schema()
        initial_tool_names = [t["name"] for t in initial_tools]
        # has_write_tools considers deferred tools that may be unlocked later
        has_write_tools = (
            any(not is_readonly_tool(tn) for tn in initial_tool_names)
            or bool(self.deferred_tools)
        )

        logger.info(
            json.dumps({
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
            }, ensure_ascii=False),
        )

        logger.info(
            json.dumps({
                "event": "agent.system_prompt",
                "conversation_id": conv_id,
                "context_id": self.context_id or "",
                "context_name": self.context_name or "",
                "context_version": self.context_version or "",
                "system_prompt": self.system_prompt,
            }, ensure_ascii=False),
        )

        async def _run_agent() -> None:
            """Main agent loop that puts SSE events into the queue."""
            nonlocal total_input_tokens, total_output_tokens, actual_turns, outcome

            try:
                for turn in range(self.max_turns):
                    actual_turns = turn + 1

                    # Start turn span
                    turn_span = _tracer.start_span("gen_ai.turn")
                    turn_span.set_attribute("gen_ai.turn.number", turn + 1)
                    turn_span.set_attribute("gen_ai.request.model", self.model)

                    if _has_otel:
                        turn_token = otel_context.attach(trace.set_span_in_context(turn_span))
                    else:
                        turn_token = None

                    try:
                        assistant_content: list[dict[str, Any]] = []
                        tool_use_blocks: list[dict[str, Any]] = []
                        current_tool_use: dict[str, Any] | None = None
                        current_tool_input_json = ""

                        # Recompute tools each turn (deferred unlock may have changed allowed_tools)
                        tools = self._get_tools_schema()
                        tool_names = [t["name"] for t in tools]

                        # Start keepalive during API call (covers initial wait)
                        _start_keepalive()

                        async with self.client.messages.stream(
                            model=self.model,
                            max_tokens=4096,
                            system=self._get_system_blocks(),
                            messages=history.get_messages(),
                            tools=tools if tools else anthropic.NOT_GIVEN,
                            temperature=self.temperature,
                        ) as stream:
                            async for event in stream:
                                if event.type == "content_block_start":
                                    # First content arriving - stop keepalive
                                    _stop_keepalive()
                                    if event.content_block.type == "tool_use":
                                        current_tool_use = {
                                            "type": "tool_use",
                                            "id": event.content_block.id,
                                            "name": event.content_block.name,
                                            "input": {},
                                        }
                                        current_tool_input_json = ""
                                elif event.type == "content_block_delta":
                                    if event.delta.type == "text_delta":
                                        await event_queue.put(
                                            sse_event("text", {"content": event.delta.text})
                                        )
                                        if assistant_content and assistant_content[-1].get("type") == "text":
                                            assistant_content[-1]["text"] += event.delta.text
                                        else:
                                            assistant_content.append({"type": "text", "text": event.delta.text})
                                    elif event.delta.type == "input_json_delta":
                                        current_tool_input_json += event.delta.partial_json
                                elif event.type == "content_block_stop":
                                    if current_tool_use:
                                        if current_tool_input_json:
                                            try:
                                                current_tool_use["input"] = json.loads(current_tool_input_json)
                                            except json.JSONDecodeError:
                                                current_tool_use["input"] = {}
                                        tool_use_blocks.append(current_tool_use)
                                        assistant_content.append(current_tool_use)
                                        current_tool_use = None
                                        current_tool_input_json = ""

                            final_message = await stream.get_final_message()

                        _stop_keepalive()

                        turn_input = final_message.usage.input_tokens
                        turn_output = final_message.usage.output_tokens
                        cache_creation = getattr(final_message.usage, "cache_creation_input_tokens", 0) or 0
                        cache_read = getattr(final_message.usage, "cache_read_input_tokens", 0) or 0
                        total_input_tokens += turn_input
                        total_output_tokens += turn_output

                        turn_span.set_attribute("gen_ai.response.finish_reason", final_message.stop_reason or "")
                        turn_span.set_attribute("gen_ai.usage.input_tokens", turn_input)
                        turn_span.set_attribute("gen_ai.usage.output_tokens", turn_output)
                        turn_span.set_attribute("gen_ai.usage.cache_creation_input_tokens", cache_creation)
                        turn_span.set_attribute("gen_ai.usage.cache_read_input_tokens", cache_read)
                        turn_span.set_attribute("gen_ai.turn.tool_call_count", len(tool_use_blocks))

                        # Collect response text for logging
                        response_text = "".join(
                            b["text"] for b in assistant_content if b.get("type") == "text"
                        )
                        logger.info(
                            json.dumps({
                                "event": "agent.turn",
                                "conversation_id": conv_id,
                                "turn_number": turn + 1,
                                "stop_reason": final_message.stop_reason or "",
                                "input_tokens": turn_input,
                                "output_tokens": turn_output,
                                "cache_creation_input_tokens": cache_creation,
                                "cache_read_input_tokens": cache_read,
                                "tool_call_count": len(tool_use_blocks),
                                "response_text": response_text,
                            }, ensure_ascii=False),
                        )

                        history.add_assistant_message(assistant_content)

                        if not tool_use_blocks:
                            # Nudge the agent to actually call tools when it
                            # responds with text-only despite having tools available.
                            # Two cases:
                            #  1. Turn 0: agent announced intent ("確認させてください")
                            #     but didn't call any tools → nudge to call read tools
                            #  2. Turn 1+: agent analysed data but didn't propose
                            #     write actions → nudge to call write tools
                            if turn == 0 and not tool_nudged and tool_names:
                                tool_nudged = True
                                history.add_user_message(
                                    "ツールを直接呼び出して情報を取得・実行してください。"
                                    "テキストでの予告は不要です。"
                                )
                                logger.info(
                                    json.dumps({
                                        "event": "agent.tool_nudge",
                                        "conversation_id": conv_id,
                                        "turn_number": turn + 1,
                                        "reason": "text_only_on_turn_0",
                                    }, ensure_ascii=False),
                                )
                                continue
                            if (
                                has_write_tools
                                and not pending_actions
                                and not write_nudged
                                and turn > 0
                            ):
                                write_nudged = True
                                history.add_user_message(
                                    "上記の分析を踏まえて、具体的なアクション（タイムブロック作成、タスク作成・更新等）を"
                                    "ツール呼び出しで提案してください。テキストでの説明は不要です。"
                                )
                                logger.info(
                                    json.dumps({
                                        "event": "agent.write_nudge",
                                        "conversation_id": conv_id,
                                        "turn_number": turn + 1,
                                    }, ensure_ascii=False),
                                )
                                continue
                            break

                        # Process tool calls: readonly = execute, write = collect as proposals
                        tool_results: list[dict[str, Any]] = []
                        has_readonly_tools = False

                        # Start keepalive during tool execution
                        _start_keepalive()

                        # Separate readonly and write tools
                        readonly_calls: list[dict[str, Any]] = []
                        write_calls: list[dict[str, Any]] = []
                        for tool_use in tool_use_blocks:
                            tool_input = dict(tool_use["input"])
                            if user_id and "user_id" not in tool_input:
                                tool_input["user_id"] = user_id
                            prepared = {**tool_use, "input": tool_input}
                            if is_readonly_tool(tool_use["name"]):
                                readonly_calls.append(prepared)
                            else:
                                write_calls.append(prepared)

                        # Execute readonly tools in parallel
                        if readonly_calls:
                            has_readonly_tools = True

                            # Send all tool_call SSE events upfront
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
                                    logger.info(
                                        json.dumps({
                                            "event": "agent.tool_call",
                                            "conversation_id": conv_id,
                                            "tool_name": tc["name"],
                                            "tool_input": json.dumps(tc["input"], ensure_ascii=False),
                                            "tool_output": result_str,
                                            "success": True,
                                        }, ensure_ascii=False),
                                    )
                                    return {
                                        "type": "tool_result",
                                        "tool_use_id": tc["id"],
                                        "content": result_str,
                                    }
                                except Exception as e:
                                    tool_span.set_attribute("tool.success", False)
                                    tool_span.set_attribute("tool.error", str(e))
                                    if _has_otel:
                                        tool_span.set_status(StatusCode.ERROR, str(e))
                                        tool_span.record_exception(e)
                                    logger.warning(
                                        json.dumps({
                                            "event": "agent.tool_call",
                                            "conversation_id": conv_id,
                                            "tool_name": tc["name"],
                                            "tool_input": json.dumps(tc["input"], ensure_ascii=False),
                                            "success": False,
                                            "error": str(e),
                                        }, ensure_ascii=False),
                                    )
                                    return {
                                        "type": "tool_result",
                                        "tool_use_id": tc["id"],
                                        "content": json.dumps({"error": str(e)}, ensure_ascii=False),
                                        "is_error": True,
                                    }
                                finally:
                                    tool_span.end()

                            parallel_results = await asyncio.gather(
                                *[_exec_readonly(tc) for tc in readonly_calls]
                            )
                            tool_results.extend(parallel_results)

                            # Send all tool_result SSE events
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
                                "content": f"この操作はユーザーの承認待ちです: {tc['name']}",
                            })

                        _stop_keepalive()

                        # Deferred unlock: readonly tool execution signals write intent
                        if has_readonly_tools and self.deferred_tools:
                            self._unlock_deferred_tools()

                        if tool_results:
                            history.add_tool_results(tool_results)

                        # If only write tools remain (no readonly executed), break to send proposal
                        if not has_readonly_tools:
                            break

                        if final_message.stop_reason == "end_turn":
                            break

                    finally:
                        _stop_keepalive()
                        if _has_otel and turn_token is not None:
                            otel_context.detach(turn_token)
                        turn_span.end()
                else:
                    # for-loop exhausted without break → max turns reached
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
                # Wait for either the agent to finish or a new event
                if agent_task.done():
                    # Drain remaining events
                    while not event_queue.empty():
                        event_str = event_queue.get_nowait()
                        if event_str is not None:
                            yield event_str
                    # Re-raise if agent failed
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

            # Record GenAI metrics
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

            logger.info(
                json.dumps({
                    "event": "agent.complete",
                    "conversation_id": conv_id,
                    "outcome": outcome,
                    "total_turns": actual_turns,
                    "total_input_tokens": total_input_tokens,
                    "total_output_tokens": total_output_tokens,
                    "pending_action_count": len(pending_actions),
                }, ensure_ascii=False),
            )

            if _has_otel and stream_token is not None:
                otel_context.detach(stream_token)
            stream_span.end()

        if agent_error is not None:
            raise agent_error

        # Yield action proposal if there are pending write actions
        if pending_actions:
            yield sse_event("action_proposal", {"actions": pending_actions})

        yield sse_event("done", {
            "conversation_id": conv_id,
            "tokens": {
                "input": total_input_tokens,
                "output": total_output_tokens,
            },
        })
