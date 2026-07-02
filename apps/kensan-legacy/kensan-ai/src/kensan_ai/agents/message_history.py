"""Message history management for agents."""

from dataclasses import dataclass, field
from typing import Any


@dataclass
class MessageHistory:
    """Manages conversation message history for agents.

    This class encapsulates the responsibility of tracking messages
    in a conversation, including user messages, assistant responses,
    and tool results.
    """

    messages: list[dict[str, Any]] = field(default_factory=list)

    def add_user_message(self, content: str) -> None:
        """Add a user message to the history.

        Args:
            content: The text content of the user's message.
        """
        self.messages.append({"role": "user", "content": content})

    def add_assistant_message(self, content: list[dict[str, Any]]) -> None:
        """Add an assistant message to the history.

        Args:
            content: List of content blocks (text and/or tool_use).
        """
        self.messages.append({"role": "assistant", "content": content})

    def add_tool_results(self, results: list[dict[str, Any]]) -> None:
        """Add tool execution results to the history.

        Tool results are added as a user message per the Anthropic API format.

        Args:
            results: List of tool result blocks.
        """
        self.messages.append({"role": "user", "content": results})

    def get_messages(self) -> list[dict[str, Any]]:
        """Get a copy of all messages in the history.

        Returns:
            A copy of the message list.
        """
        return self.messages.copy()

    def clear(self) -> None:
        """Clear all messages from the history."""
        self.messages.clear()

    def __len__(self) -> int:
        """Return the number of messages in the history."""
        return len(self.messages)

    def is_empty(self) -> bool:
        """Check if the history is empty.

        Returns:
            True if there are no messages, False otherwise.
        """
        return len(self.messages) == 0
