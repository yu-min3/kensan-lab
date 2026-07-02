"""Kensan AI - AI agents for learning management.

Usage:
    # Run as FastAPI server:
    uvicorn kensan_ai.main:app --reload

    # Or use agents directly:
    from kensan_ai.agents import AgentRunner, chat

    agent = AgentRunner(
        system_prompt=chat.SYSTEM_PROMPT,
        allowed_tools=chat.ALLOWED_TOOLS,
    )
    result = await agent.run("今日のタスクを教えて", user_id="...")
"""

from kensan_ai.main import app, create_app
from kensan_ai.agents import AgentRunner, AgentResult

__all__ = [
    "app",
    "create_app",
    "AgentRunner",
    "AgentResult",
]
