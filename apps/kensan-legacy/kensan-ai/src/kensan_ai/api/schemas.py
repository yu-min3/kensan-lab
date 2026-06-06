"""Pydantic models for API request/response."""

from datetime import datetime
from typing import Literal

from pydantic import BaseModel, Field


# =============================================================================
# Health & Feedback
# =============================================================================

class HealthResponse(BaseModel):
    """Health check response."""

    status: str = "ok"
    version: str = "0.1.0"


class FeedbackRequest(BaseModel):
    """Request for interaction feedback endpoint."""

    rating: int = Field(..., ge=1, le=5, description="Rating from 1-5")
    feedback: str | None = Field(None, description="Optional text feedback")


class FeedbackResponse(BaseModel):
    """Response from feedback endpoint."""

    success: bool
    message: str


# =============================================================================
# Unified Agent Endpoints
# =============================================================================

class AgentStreamRequest(BaseModel):
    """Request for unified agent stream endpoint."""

    message: str = Field(..., description="User's message")
    conversation_id: str | None = Field(None, description="Conversation ID for continuity")
    situation: Literal["auto", "review", "chat", "daily_advice"] = Field(
        "auto", description="Situation context"
    )
    context: dict[str, str] | None = Field(
        None,
        description="Pre-fetched data from frontend to inject into system prompt, reducing tool calls",
    )
    context_id: str | None = Field(
        None,
        description="Direct AI context ID to use (bypasses situation detection)",
    )
    version_number: int | None = Field(
        None,
        description="Specific version number to use for the context (for A/B comparison)",
    )


class AgentApproveRequest(BaseModel):
    """Request for agent action approval endpoint."""

    conversation_id: str = Field(..., description="Conversation ID with pending actions")
    action_ids: list[str] = Field(..., description="IDs of actions to approve")


class AgentRejectRequest(BaseModel):
    """Request for agent action rejection endpoint."""

    conversation_id: str = Field(..., description="Conversation ID with pending actions")


# =============================================================================
# Prompt Metadata
# =============================================================================

class VariableMetadataItem(BaseModel):
    """Metadata for a system prompt variable."""

    name: str
    description: str
    example: str
    excludes_tools: list[str] = []


class ToolMetadataItem(BaseModel):
    """Metadata for an allowed tool."""

    name: str
    description: str
    readonly: bool
    category: str


class PromptMetadataResponse(BaseModel):
    """Response for prompt metadata (variables + tools)."""

    variables: list[VariableMetadataItem]
    tools: list[ToolMetadataItem]


# =============================================================================
# Prompt Management
# =============================================================================

class AIContextResponse(BaseModel):
    """Response for an AI context."""

    id: str
    name: str
    situation: str
    version: str
    is_active: bool
    is_default: bool
    system_prompt: str
    allowed_tools: list[str]
    max_turns: int
    temperature: float
    description: str | None = None
    created_at: str
    updated_at: str
    current_version_number: int | None = None
    active_version: int | None = None
    pending_candidate_count: int = 0


class AIContextUpdateRequest(BaseModel):
    """Request to update an AI context."""

    system_prompt: str | None = None
    allowed_tools: list[str] | None = None
    max_turns: int | None = None
    temperature: float | None = None
    changelog: str | None = None


class AIContextVersionResponse(BaseModel):
    """Response for an AI context version."""

    id: str
    context_id: str
    version_number: int
    system_prompt: str
    allowed_tools: list[str]
    max_turns: int
    temperature: float
    changelog: str | None = None
    created_at: str
    source: str = "manual"
    eval_summary: dict | None = None
    candidate_status: str | None = None


class ConversationRateRequest(BaseModel):
    """Request to rate a conversation."""

    rating: int = Field(..., ge=1, le=5, description="Rating from 1-5")
    feedback: str | None = Field(None, description="Optional text feedback")
