"""In-memory conversation store for agent approval flow."""

from dataclasses import dataclass, field
from datetime import datetime, timedelta
from typing import Any
from uuid import UUID

from kensan_ai.agents.message_history import MessageHistory


@dataclass
class PendingAction:
    """A pending write action awaiting user approval."""

    id: str
    tool_name: str
    description: str
    input: dict[str, Any]


@dataclass
class ConversationState:
    """State of an active conversation."""

    conversation_id: str
    user_id: UUID
    pending_actions: list[PendingAction] = field(default_factory=list)
    message_history: MessageHistory = field(default_factory=MessageHistory)
    created_at: datetime = field(default_factory=datetime.utcnow)
    last_activity_at: datetime = field(default_factory=datetime.utcnow)


class ConversationStore:
    """In-memory conversation store with TTL-based expiration.

    Stores pending actions and message history for active conversations.
    For production scaling, this could be replaced with Redis.
    """

    def __init__(self, ttl_minutes: int = 30):
        self._store: dict[str, ConversationState] = {}
        self._ttl = timedelta(minutes=ttl_minutes)

    def get(self, conversation_id: str) -> ConversationState | None:
        """Get conversation state by ID, returns None if expired or not found."""
        state = self._store.get(conversation_id)
        if state is None:
            return None
        if datetime.utcnow() - state.last_activity_at > self._ttl:
            del self._store[conversation_id]
            return None
        # Update activity timestamp on access
        state.last_activity_at = datetime.utcnow()
        return state

    def set(self, state: ConversationState) -> None:
        """Store or update a conversation state."""
        state.last_activity_at = datetime.utcnow()
        self._store[state.conversation_id] = state

    def remove(self, conversation_id: str) -> None:
        """Remove a conversation state."""
        self._store.pop(conversation_id, None)

    def cleanup_expired(self) -> int:
        """Remove all expired conversation states.

        Returns:
            Number of expired states removed.
        """
        now = datetime.utcnow()
        expired = [
            cid for cid, state in self._store.items()
            if now - state.last_activity_at > self._ttl
        ]
        for cid in expired:
            del self._store[cid]
        return len(expired)
