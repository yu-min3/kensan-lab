"""Database connection management using asyncpg."""

from contextlib import asynccontextmanager
from typing import AsyncIterator

import asyncpg
from pgvector.asyncpg import register_vector

from kensan_ai.config import get_settings

_pool: asyncpg.Pool | None = None


async def _init_connection(conn: asyncpg.Connection) -> None:
    """Initialize each new connection with pgvector type support."""
    await register_vector(conn)


async def create_pool() -> asyncpg.Pool:
    """Create a new connection pool."""
    settings = get_settings()
    return await asyncpg.create_pool(
        settings.effective_database_url,
        min_size=2,
        max_size=10,
        init=_init_connection,
    )


async def get_pool() -> asyncpg.Pool:
    """Get the connection pool, creating it if necessary."""
    global _pool
    if _pool is None:
        _pool = await create_pool()
    return _pool


async def close_pool() -> None:
    """Close the connection pool."""
    global _pool
    if _pool is not None:
        await _pool.close()
        _pool = None


@asynccontextmanager
async def get_connection() -> AsyncIterator[asyncpg.Connection]:
    """Get a database connection from the pool."""
    pool = await get_pool()
    async with pool.acquire() as connection:
        yield connection
