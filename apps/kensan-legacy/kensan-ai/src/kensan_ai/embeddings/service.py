"""Embedding service for generating vector embeddings using OpenAI or Gemini."""

import logging
from typing import Any

from kensan_ai.config import get_settings

logger = logging.getLogger(__name__)


class EmbeddingService:
    """Service for generating text embeddings using OpenAI or Gemini API."""

    # Embedding dimensions for different models
    MODEL_DIMENSIONS = {
        "text-embedding-3-small": 1536,
        "text-embedding-3-large": 3072,
        "text-embedding-ada-002": 1536,
    }

    # Gemini uses output_dimensionality to control dimensions
    GEMINI_DIMENSIONS = 1536

    def __init__(self):
        """Initialize the embedding service."""
        settings = get_settings()
        self.provider = settings.embedding_provider  # "openai" or "gemini"

        if self.provider == "gemini":
            self.model = settings.gemini_embedding_model
            self._google_api_key = settings.google_api_key
        else:
            self.model = settings.embedding_model
            self.api_key = settings.openai_api_key

        self._client: Any = None
        self._gemini_client: Any = None

    @property
    def dimensions(self) -> int:
        """Get the embedding dimensions for the configured model."""
        if self.provider == "gemini":
            return self.GEMINI_DIMENSIONS
        return self.MODEL_DIMENSIONS.get(self.model, 1536)

    async def _get_client(self) -> Any:
        """Get or create the OpenAI async client."""
        if self._client is None:
            try:
                from openai import AsyncOpenAI
                self._client = AsyncOpenAI(api_key=self.api_key)
            except ImportError:
                raise ImportError(
                    "openai package is required for embeddings. "
                    "Install with: pip install openai"
                )
        return self._client

    def _get_gemini_client(self) -> Any:
        """Get or create the Gemini client (lazy init)."""
        if self._gemini_client is None:
            try:
                from google import genai
                self._gemini_client = genai.Client(api_key=self._google_api_key)
            except ImportError:
                raise ImportError(
                    "google-genai package is required for Gemini embeddings. "
                    "Install with: pip install google-genai"
                )
        return self._gemini_client

    def _generate_gemini_embedding(
        self, text: str, task_type: str = "RETRIEVAL_DOCUMENT"
    ) -> list[float]:
        """Generate a single embedding using Gemini API (synchronous)."""
        client = self._get_gemini_client()
        response = client.models.embed_content(
            model=self.model,
            contents=text,
            config={
                "output_dimensionality": self.GEMINI_DIMENSIONS,
                "task_type": task_type,
            },
        )
        return list(response.embeddings[0].values)

    def _generate_gemini_embeddings(
        self, texts: list[str], task_type: str = "RETRIEVAL_DOCUMENT"
    ) -> list[list[float]]:
        """Generate batch embeddings using Gemini API (synchronous)."""
        client = self._get_gemini_client()
        response = client.models.embed_content(
            model=self.model,
            contents=texts,
            config={
                "output_dimensionality": self.GEMINI_DIMENSIONS,
                "task_type": task_type,
            },
        )
        return [list(e.values) for e in response.embeddings]

    async def generate_embedding(
        self, text: str, *, task_type: str | None = None
    ) -> list[float]:
        """Generate an embedding vector for the given text.

        Args:
            text: The text to generate an embedding for
            task_type: Gemini task type (e.g. "RETRIEVAL_QUERY", "RETRIEVAL_DOCUMENT").
                       Ignored for OpenAI. Defaults to "RETRIEVAL_DOCUMENT" for Gemini.

        Returns:
            A list of floats representing the embedding vector

        Raises:
            ValueError: If the text is empty
            RuntimeError: If the API call fails
        """
        if not text or not text.strip():
            raise ValueError("Text cannot be empty")

        # Truncate text if too long
        # Approximate: 1 token ≈ 4 chars for English, less for Japanese
        max_chars = 8000 * 2  # Conservative estimate for Japanese
        if len(text) > max_chars:
            text = text[:max_chars]
            logger.warning(f"Text truncated to {max_chars} characters for embedding")

        try:
            if self.provider == "gemini":
                gemini_task = task_type or "RETRIEVAL_DOCUMENT"
                return self._generate_gemini_embedding(text, task_type=gemini_task)

            client = await self._get_client()
            response = await client.embeddings.create(
                input=text,
                model=self.model,
            )
            return response.data[0].embedding
        except Exception as e:
            logger.error(f"Failed to generate embedding: {e}")
            raise RuntimeError(f"Embedding generation failed: {e}") from e

    async def generate_embeddings(
        self, texts: list[str], *, task_type: str | None = None
    ) -> list[list[float]]:
        """Generate embeddings for multiple texts in a batch.

        Args:
            texts: List of texts to generate embeddings for
            task_type: Gemini task type. Ignored for OpenAI.
                       Defaults to "RETRIEVAL_DOCUMENT" for Gemini.

        Returns:
            List of embedding vectors

        Raises:
            ValueError: If texts list is empty or contains empty strings
            RuntimeError: If the API call fails
        """
        if not texts:
            raise ValueError("Texts list cannot be empty")

        # Filter and truncate
        max_chars = 8000 * 2
        processed_texts = []
        for text in texts:
            if not text or not text.strip():
                processed_texts.append(" ")  # Requires non-empty
            elif len(text) > max_chars:
                processed_texts.append(text[:max_chars])
            else:
                processed_texts.append(text)

        try:
            if self.provider == "gemini":
                gemini_task = task_type or "RETRIEVAL_DOCUMENT"
                return self._generate_gemini_embeddings(
                    processed_texts, task_type=gemini_task
                )

            client = await self._get_client()
            response = await client.embeddings.create(
                input=processed_texts,
                model=self.model,
            )
            # Sort by index to ensure correct order
            sorted_data = sorted(response.data, key=lambda x: x.index)
            return [item.embedding for item in sorted_data]
        except Exception as e:
            logger.error(f"Failed to generate embeddings: {e}")
            raise RuntimeError(f"Batch embedding generation failed: {e}") from e


# Global instance for reuse
_service: EmbeddingService | None = None


def get_embedding_service() -> EmbeddingService:
    """Get or create the global EmbeddingService instance."""
    global _service
    if _service is None:
        _service = EmbeddingService()
    return _service
