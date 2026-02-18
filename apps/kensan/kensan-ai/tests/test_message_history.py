"""Tests for MessageHistory class."""

import pytest

from kensan_ai.agents.message_history import MessageHistory


class TestMessageHistory:
    """Tests for MessageHistory."""

    def test_init_empty(self):
        """Test initialization creates empty history."""
        history = MessageHistory()
        assert len(history) == 0
        assert history.is_empty()

    def test_add_user_message(self):
        """Test adding a user message."""
        history = MessageHistory()
        history.add_user_message("Hello, world!")

        assert len(history) == 1
        assert not history.is_empty()

        messages = history.get_messages()
        assert messages[0]["role"] == "user"
        assert messages[0]["content"] == "Hello, world!"

    def test_add_assistant_message(self):
        """Test adding an assistant message."""
        history = MessageHistory()
        content = [{"type": "text", "text": "Hello!"}]
        history.add_assistant_message(content)

        assert len(history) == 1
        messages = history.get_messages()
        assert messages[0]["role"] == "assistant"
        assert messages[0]["content"] == content

    def test_add_tool_results(self):
        """Test adding tool results."""
        history = MessageHistory()
        results = [
            {
                "type": "tool_result",
                "tool_use_id": "tool_123",
                "content": '{"result": "success"}',
            }
        ]
        history.add_tool_results(results)

        assert len(history) == 1
        messages = history.get_messages()
        assert messages[0]["role"] == "user"
        assert messages[0]["content"] == results

    def test_get_messages_returns_copy(self):
        """Test that get_messages returns a copy, not the original."""
        history = MessageHistory()
        history.add_user_message("Test")

        messages1 = history.get_messages()
        messages2 = history.get_messages()

        assert messages1 == messages2
        assert messages1 is not messages2

        # Modifying the returned list shouldn't affect the original
        messages1.append({"role": "user", "content": "Modified"})
        assert len(history) == 1

    def test_clear(self):
        """Test clearing the history."""
        history = MessageHistory()
        history.add_user_message("Message 1")
        history.add_user_message("Message 2")

        assert len(history) == 2

        history.clear()

        assert len(history) == 0
        assert history.is_empty()

    def test_conversation_flow(self):
        """Test a typical conversation flow."""
        history = MessageHistory()

        # User sends message
        history.add_user_message("What's the weather?")

        # Assistant responds with tool use
        assistant_content = [
            {"type": "text", "text": "Let me check the weather."},
            {
                "type": "tool_use",
                "id": "tool_1",
                "name": "get_weather",
                "input": {"location": "Tokyo"},
            },
        ]
        history.add_assistant_message(assistant_content)

        # Tool results
        tool_results = [
            {
                "type": "tool_result",
                "tool_use_id": "tool_1",
                "content": '{"temp": 20, "condition": "sunny"}',
            }
        ]
        history.add_tool_results(tool_results)

        # Final assistant response
        final_content = [{"type": "text", "text": "It's 20 degrees and sunny in Tokyo."}]
        history.add_assistant_message(final_content)

        assert len(history) == 4
        messages = history.get_messages()

        assert messages[0]["role"] == "user"
        assert messages[1]["role"] == "assistant"
        assert messages[2]["role"] == "user"  # Tool results are user messages
        assert messages[3]["role"] == "assistant"

    def test_len_dunder(self):
        """Test __len__ method."""
        history = MessageHistory()
        assert len(history) == 0

        history.add_user_message("Test 1")
        assert len(history) == 1

        history.add_user_message("Test 2")
        assert len(history) == 2
