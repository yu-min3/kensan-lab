"""Kensan AI Agents - Agent definitions with system prompts.

To add a new agent:
1. Create a new file (e.g., morning_planner.py)
2. Define SYSTEM_PROMPT and ALLOWED_TOOLS
3. Import and export from this file
"""

from kensan_ai.agents.base import AgentRunner, AgentResult, ToolCall
from kensan_ai.agents.gemini_runner import GeminiAgentRunner
from kensan_ai.agents.message_history import MessageHistory
from kensan_ai.agents import chat
from kensan_ai.config import get_settings


def create_agent_runner(**kwargs) -> AgentRunner | GeminiAgentRunner:
    """Factory function to create the appropriate agent runner based on ai_provider setting."""
    settings = get_settings()
    if settings.ai_provider == "google-adk":
        from kensan_ai.agents.adk_runner import AdkAgentRunner
        return AdkAgentRunner(**kwargs)
    if settings.ai_provider == "google":
        return GeminiAgentRunner(**kwargs)
    return AgentRunner(**kwargs)


__all__ = [
    "AgentRunner",
    "GeminiAgentRunner",
    "AgentResult",
    "ToolCall",
    "MessageHistory",
    "create_agent_runner",
    "chat",
]
