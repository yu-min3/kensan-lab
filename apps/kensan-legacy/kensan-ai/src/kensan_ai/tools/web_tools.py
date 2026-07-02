"""External Web Tools - Web search and fetch via Tavily API.

Results are optionally recorded to the Lakehouse Bronze layer (fire & forget).
"""

import asyncio
import json
import logging
from typing import Any

from kensan_ai.config import get_settings
from kensan_ai.tools.base import tool

logger = logging.getLogger(__name__)


def _get_tavily_client():
    """Get Tavily client (lazy import to avoid import errors when not installed)."""
    from tavily import TavilyClient

    settings = get_settings()
    if not settings.tavily_api_key:
        raise ValueError("TAVILY_API_KEY is not configured")
    return TavilyClient(api_key=settings.tavily_api_key)


async def _fire_and_forget_lakehouse(
    tool_name: str,
    input_data: str,
    result_json: str,
    result_count: int,
    metadata: dict[str, Any] | None = None,
):
    """Fire & forget でLakehouseに書き込む。エラーは無視。"""
    try:
        from kensan_ai.lakehouse.writer import get_writer

        writer = get_writer()
        await writer.append_tool_result(
            tool_name=tool_name,
            input_data=input_data,
            result_json=result_json,
            result_count=result_count,
            metadata=metadata,
        )
    except Exception as e:
        logger.debug(f"Lakehouse fire-and-forget failed (ignored): {e}")


@tool(
    category="web",
    name="web_search",
    description="Web検索を実行して最新の情報を取得する。技術ドキュメント、ニュース、ブログ記事などの検索に使用。",
    input_schema={
        "properties": {
            "query": {"type": "string", "description": "検索クエリ"},
            "max_results": {
                "type": "integer",
                "description": "最大結果数 (デフォルト: 5)",
            },
            "search_depth": {
                "type": "string",
                "description": "basic または advanced (デフォルト: basic)",
                "enum": ["basic", "advanced"],
            },
        },
        "required": ["query"],
    },
)
async def web_search(args: dict[str, Any]) -> dict[str, Any]:
    """Web検索を実行する。"""
    query = args["query"]
    max_results = args.get("max_results", 5)
    search_depth = args.get("search_depth", "basic")

    # Run Tavily search in executor (blocking I/O)
    loop = asyncio.get_event_loop()
    client = _get_tavily_client()
    response = await loop.run_in_executor(
        None,
        lambda: client.search(
            query=query,
            max_results=max_results,
            search_depth=search_depth,
        ),
    )

    # Format results
    results = []
    for item in response.get("results", []):
        results.append({
            "title": item.get("title", ""),
            "url": item.get("url", ""),
            "content": item.get("content", ""),
            "score": item.get("score"),
        })

    formatted = {
        "query": query,
        "results": results,
        "result_count": len(results),
    }

    # Fire & forget lakehouse write
    asyncio.create_task(
        _fire_and_forget_lakehouse(
            tool_name="web_search",
            input_data=query,
            result_json=json.dumps(formatted, ensure_ascii=False),
            result_count=len(results),
            metadata={"search_depth": search_depth, "max_results": max_results},
        )
    )

    return formatted


@tool(
    category="web",
    name="web_fetch",
    description="指定URLのWebページからコンテンツを取得・抽出する。ドキュメントや記事の詳細を読むときに使用。",
    input_schema={
        "properties": {
            "url": {"type": "string", "description": "取得するURL"},
        },
        "required": ["url"],
    },
)
async def web_fetch(args: dict[str, Any]) -> dict[str, Any]:
    """指定URLのWebページからコンテンツを取得する。"""
    url = args["url"]

    # Run Tavily extract in executor (blocking I/O)
    loop = asyncio.get_event_loop()
    client = _get_tavily_client()
    response = await loop.run_in_executor(
        None,
        lambda: client.extract(urls=[url]),
    )

    # Format results
    extracted = response.get("results", [])
    if extracted:
        content = extracted[0].get("raw_content", "") or extracted[0].get("content", "")
        result = {
            "url": url,
            "content": content[:10000],  # Limit to 10k chars
            "content_length": len(content),
        }
    else:
        result = {
            "url": url,
            "content": "",
            "content_length": 0,
            "error": "コンテンツを取得できませんでした",
        }

    # Fire & forget lakehouse write
    asyncio.create_task(
        _fire_and_forget_lakehouse(
            tool_name="web_fetch",
            input_data=url,
            result_json=json.dumps(result, ensure_ascii=False),
            result_count=1 if result.get("content") else 0,
        )
    )

    return result


ALL_WEB_TOOLS = ["web_search", "web_fetch"]
