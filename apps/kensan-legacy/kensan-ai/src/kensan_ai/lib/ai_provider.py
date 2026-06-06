"""Shared AI provider factory for LLM calls."""

from __future__ import annotations

import logging
from typing import Any

import anthropic
from google import genai
from google.genai import errors as genai_errors

from kensan_ai.config import get_settings

logger = logging.getLogger(__name__)


class LLMClient:
    """Unified LLM client wrapping Anthropic and Google GenAI."""

    def __init__(self) -> None:
        settings = get_settings()
        self.ai_provider = settings.ai_provider

        if self.ai_provider == "google":
            self.google_client = genai.Client(api_key=settings.google_api_key)
            self.model = settings.google_model
        else:
            self.anthropic_client = anthropic.AsyncAnthropic(
                api_key=settings.anthropic_api_key
            )
            self.model = settings.anthropic_model

    async def generate(
        self,
        prompt: str,
        *,
        max_tokens: int = 8192,
        system: str | None = None,
    ) -> str:
        """Generate a response from the LLM.

        Args:
            prompt: The user message / contents.
            max_tokens: Maximum output tokens (Anthropic only).
            system: Optional system instruction.

        Returns:
            The generated text content.

        Raises:
            anthropic.APIError: On Anthropic API failures.
            genai_errors.APIError: On Google GenAI API failures.
        """
        if self.ai_provider == "google":
            config: dict[str, Any] = {}
            if system:
                config["system_instruction"] = system
            response = await self.google_client.aio.models.generate_content(
                model=self.model,
                contents=prompt,
                **({"config": config} if config else {}),
            )
            return response.text or ""
        else:
            kwargs: dict[str, Any] = {
                "model": self.model,
                "max_tokens": max_tokens,
                "messages": [{"role": "user", "content": prompt}],
            }
            if system:
                kwargs["system"] = system
            response = await self.anthropic_client.messages.create(**kwargs)
            return response.content[0].text if response.content else ""
