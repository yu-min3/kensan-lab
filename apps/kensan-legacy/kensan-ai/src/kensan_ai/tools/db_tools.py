"""Database tools for Direct Tools approach."""

from collections import defaultdict
from datetime import datetime, timedelta
from typing import Any
from zoneinfo import ZoneInfo

from kensan_ai.tools.base import tool
from kensan_ai.db.queries import (
    get_goals_and_milestones as db_get_goals,
    get_tasks as db_get_tasks,
    create_task as db_create_task,
    update_task as db_update_task,
    delete_task as db_delete_task,
    get_time_blocks as db_get_time_blocks,
    create_time_block as db_create_time_block,
    update_time_block as db_update_time_block,
    delete_time_block as db_delete_time_block,
    get_time_entries as db_get_time_entries,
    get_memos as db_get_memos,
    create_memo as db_create_memo,
    get_notes as db_get_notes,
    create_note as db_create_note,
    update_note as db_update_note,
    create_goal as db_create_goal,
    update_goal as db_update_goal,
    delete_goal as db_delete_goal,
    create_milestone as db_create_milestone,
    update_milestone as db_update_milestone,
    delete_milestone as db_delete_milestone,
)
from kensan_ai.db.queries.user_settings import get_user_timezone
from kensan_ai.lib.parsers import parse_uuid, parse_date, parse_time
from kensan_ai.lib.timezone_utils import (
    local_date_to_utc_range as _local_date_to_utc_range,
    combine_to_utc as _combine_to_utc,
    to_local_str as _to_local,
)


def _compute_free_slots(
    blocks: list[dict[str, Any]],
    tz: ZoneInfo,
    day_start_hour: int = 6,
    day_end_hour: int = 23,
) -> dict[str, list[str]]:
    """Compute free time slots per day from a list of time blocks.

    Returns a dict like {"2026-02-10": ["06:00-09:00", "10:30-14:00", "16:00-23:00"]}.
    """
    # Group blocks by local date
    by_date: dict[str, list[tuple[int, int]]] = defaultdict(list)
    for b in blocks:
        start_iso = b.get("startDatetime") or b.get("start_datetime")
        end_iso = b.get("endDatetime") or b.get("end_datetime")
        if not start_iso or not end_iso:
            continue
        start_dt = datetime.fromisoformat(start_iso).astimezone(tz)
        end_dt = datetime.fromisoformat(end_iso).astimezone(tz)
        local_date = start_dt.strftime("%Y-%m-%d")
        start_min = start_dt.hour * 60 + start_dt.minute
        end_min = end_dt.hour * 60 + end_dt.minute
        by_date[local_date].append((start_min, end_min))

    result: dict[str, list[str]] = {}
    for date_str, intervals in sorted(by_date.items()):
        # Merge overlapping intervals
        intervals.sort()
        merged: list[tuple[int, int]] = []
        for s, e in intervals:
            if merged and s <= merged[-1][1]:
                merged[-1] = (merged[-1][0], max(merged[-1][1], e))
            else:
                merged.append((s, e))

        # Compute free slots between day_start and day_end
        free: list[str] = []
        cursor = day_start_hour * 60
        day_end_min = day_end_hour * 60
        for busy_start, busy_end in merged:
            if cursor < busy_start and cursor < day_end_min:
                free_end = min(busy_start, day_end_min)
                free.append(f"{cursor // 60:02d}:{cursor % 60:02d}-{free_end // 60:02d}:{free_end % 60:02d}")
            cursor = max(cursor, busy_end)
        if cursor < day_end_min:
            free.append(f"{cursor // 60:02d}:{cursor % 60:02d}-{day_end_min // 60:02d}:{day_end_min % 60:02d}")

        result[date_str] = free

    return result


@tool(
    category="db",
    name="get_goals_and_milestones",
    description="目標とマイルストーンの一覧を取得する。各目標には紐づくマイルストーンとタスクの完了状況が含まれる。",
    input_schema={
        "properties": {},
        "required": [],
    },
)
async def get_goals_and_milestones(args: dict[str, Any]) -> dict[str, Any]:
    """Get all goals with their milestones and task counts."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    goals = await db_get_goals(user_id)
    # Slim down: remove descriptions to reduce token count
    return {"goals": [
        {
            "id": g["id"],
            "name": g["name"],
            "color": g["color"],
            "milestones": [
                {
                    "id": m["id"],
                    "name": m["name"],
                    "targetDate": m.get("targetDate"),
                    "status": m["status"],
                    "taskCount": m["taskCount"],
                }
                for m in g.get("milestones", [])
            ],
        }
        for g in goals
    ]}


@tool(
    category="db",
    name="get_tasks",
    description="タスク一覧を取得する。マイルストーンや完了状態、期日でフィルタできる。タスク関連の操作前には必ずこのツールで既存タスクを確認すること。",
    input_schema={
        "properties": {
            "milestone_id": {
                "type": "string",
                "description": "マイルストーンID (UUID形式、省略可)",
            },
            "completed": {
                "type": "boolean",
                "description": "完了状態でフィルタ (省略可)",
            },
            "due_date": {
                "type": "string",
                "description": "期日 (YYYY-MM-DD形式、省略可)",
            },
        },
        "required": [],
    },
)
async def get_tasks(args: dict[str, Any]) -> dict[str, Any]:
    """Get tasks with optional filters."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    tasks = await db_get_tasks(
        user_id=user_id,
        milestone_id=parse_uuid(args.get("milestone_id")),
        completed=args.get("completed"),
        due_date=parse_date(args.get("due_date")),
    )
    return {"tasks": tasks}


@tool(
    category="db",
    name="create_task",
    description="新しいタスクを作成する。使用前に必ず get_tasks で既存タスクを検索し、同名・類似のタスクがないか確認すること。既存タスクがあればそちらを使う。",
    readonly=False,
    input_schema={
        "properties": {
            "name": {
                "type": "string",
                "description": "タスク名",
            },
            "milestone_id": {
                "type": "string",
                "description": "マイルストーンID (UUID形式、省略可)",
            },
            "estimated_minutes": {
                "type": "integer",
                "description": "見積もり時間(分、省略可)",
            },
            "due_date": {
                "type": "string",
                "description": "期日 (YYYY-MM-DD形式、省略可)",
            },
        },
        "required": ["name"],
    },
)
async def create_task(args: dict[str, Any]) -> dict[str, Any]:
    """Create a new task."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    name = args.get("name")
    if not name:
        return {"error": "Missing task name"}

    task = await db_create_task(
        user_id=user_id,
        name=name,
        milestone_id=parse_uuid(args.get("milestone_id")),
        estimated_minutes=args.get("estimated_minutes"),
        due_date=parse_date(args.get("due_date")),
    )
    return {"task": task}


@tool(
    category="db",
    name="update_task",
    description="既存タスクを更新する。完了マーク、名前変更、期限変更に使う。事前に get_tasks でタスクIDを特定すること。",
    readonly=False,
    input_schema={
        "properties": {
            "task_id": {
                "type": "string",
                "description": "タスクID (UUID形式)",
            },
            "name": {
                "type": "string",
                "description": "新しいタスク名 (省略可)",
            },
            "completed": {
                "type": "boolean",
                "description": "完了状態 (省略可)",
            },
            "due_date": {
                "type": "string",
                "description": "期日 (YYYY-MM-DD形式、省略可)",
            },
        },
        "required": ["task_id"],
    },
)
async def update_task(args: dict[str, Any]) -> dict[str, Any]:
    """Update an existing task."""
    user_id = parse_uuid(args.get("user_id"))
    task_id = parse_uuid(args.get("task_id"))
    if not user_id or not task_id:
        return {"error": "Invalid or missing user_id or task_id"}

    task = await db_update_task(
        task_id=task_id,
        user_id=user_id,
        name=args.get("name"),
        completed=args.get("completed"),
        due_date=parse_date(args.get("due_date")),
    )
    if task is None:
        return {"error": "Task not found or no updates provided"}
    return {"task": task}


@tool(
    category="db",
    name="get_time_blocks",
    description="タイムブロック（計画）を取得する。日付や期間でフィルタできる。タイムブロック操作前には必ずこのツールで既存の予定を確認すること。返却の freeSlots に日ごとの空き時間帯が含まれるので、create_time_block ではその範囲内に配置すること。",
    input_schema={
        "properties": {
            "date": {
                "type": "string",
                "description": "特定の日付 (YYYY-MM-DD形式、省略可)",
            },
            "start_date": {
                "type": "string",
                "description": "期間の開始日 (YYYY-MM-DD形式、省略可)",
            },
            "end_date": {
                "type": "string",
                "description": "期間の終了日 (YYYY-MM-DD形式、省略可)",
            },
        },
        "required": [],
    },
)
async def get_time_blocks(args: dict[str, Any]) -> dict[str, Any]:
    """Get time blocks with optional date filters."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    user_tz = await get_user_timezone(user_id)

    # Convert local date filters to UTC datetime range
    start_dt = None
    end_dt = None
    target_date = parse_date(args.get("date"))
    if target_date is not None:
        start_dt, end_dt = _local_date_to_utc_range(target_date, user_tz)
    else:
        start_date = parse_date(args.get("start_date"))
        end_date = parse_date(args.get("end_date"))
        if start_date is not None and end_date is not None:
            start_dt, _ = _local_date_to_utc_range(start_date, user_tz)
            _, end_dt = _local_date_to_utc_range(end_date, user_tz)

    blocks = await db_get_time_blocks(
        user_id=user_id,
        start_datetime=start_dt,
        end_datetime=end_dt,
    )
    # Slim down: flatten nested objects, remove IDs not needed for reading
    # Convert UTC datetimes to user's local timezone
    slim_blocks = [
        {
            "id": b["id"],
            "startDatetime": _to_local(b["startDatetime"], user_tz),
            "endDatetime": _to_local(b["endDatetime"], user_tz),
            "taskName": b["taskName"],
            "goalName": b["goal"]["name"] if b.get("goal") else None,
            "milestoneName": b["milestone"]["name"] if b.get("milestone") else None,
        }
        for b in blocks
    ]

    # Compute free slots per day so the LLM can avoid conflicts
    free_slots = _compute_free_slots(blocks, user_tz)

    return {"timeBlocks": slim_blocks, "freeSlots": free_slots}


@tool(
    category="db",
    name="create_time_block",
    description="タイムブロック（計画）を作成する。手順: (1) get_tasks で既存タスクを確認しtask_id等を取得 (2) get_time_blocks でその日の予定を確認し freeSlots を参照 (3) freeSlots の範囲内に収まる時間帯を選んで作成。既存タスクが見つかれば task_id, milestone_id, goal_id を紐付ける（goal_name/goal_color/milestone_name はIDから自動解決される）。ユーザーの時間指定が曖昧な場合（「朝」「昼」「午後」等）は常識的に見繕う（朝→08:00-09:00、昼→12:00-13:00、午後→14:00-15:00、夕方→17:00-18:00）。freeSlots と重なる空き時間がない場合はユーザーに確認すること。",
    readonly=False,
    input_schema={
        "properties": {
            "date": {
                "type": "string",
                "description": "日付 (YYYY-MM-DD形式)",
            },
            "start_time": {
                "type": "string",
                "description": "開始時刻 (HH:MM形式)",
            },
            "end_time": {
                "type": "string",
                "description": "終了時刻 (HH:MM形式)",
            },
            "task_name": {
                "type": "string",
                "description": "タスク名",
            },
            "task_id": {
                "type": "string",
                "description": "タスクID (UUID形式、省略可。既存タスクがあれば必ず指定)",
            },
            "milestone_id": {
                "type": "string",
                "description": "マイルストーンID (UUID形式、省略可)",
            },
            "goal_id": {
                "type": "string",
                "description": "目標ID (UUID形式、省略可)",
            },
        },
        "required": ["date", "start_time", "end_time", "task_name"],
    },
)
async def create_time_block(args: dict[str, Any]) -> dict[str, Any]:
    """Create a new time block."""
    user_id = parse_uuid(args.get("user_id"))
    target_date = parse_date(args.get("date"))
    start_time = parse_time(args.get("start_time"))
    end_time = parse_time(args.get("end_time"))
    task_name = args.get("task_name")

    if not user_id:
        return {"error": "Invalid or missing user_id"}
    if not target_date:
        return {"error": "Invalid or missing date"}
    if not start_time or not end_time:
        return {"error": "Invalid or missing start_time or end_time"}
    if not task_name:
        return {"error": "Missing task_name"}

    user_tz = await get_user_timezone(user_id)

    # Combine local date + time into UTC datetimes
    start_dt = _combine_to_utc(target_date, start_time, user_tz)
    end_dt = _combine_to_utc(target_date, end_time, user_tz)
    # Handle overnight blocks (e.g., 23:30 - 00:30)
    if end_dt <= start_dt:
        end_dt += timedelta(days=1)

    block = await db_create_time_block(
        user_id=user_id,
        start_datetime=start_dt,
        end_datetime=end_dt,
        task_name=task_name,
        task_id=parse_uuid(args.get("task_id")),
        milestone_id=parse_uuid(args.get("milestone_id")),
        goal_id=parse_uuid(args.get("goal_id")),
    )
    # Convert UTC datetimes to user's local timezone
    block["startDatetime"] = _to_local(block["startDatetime"], user_tz)
    block["endDatetime"] = _to_local(block["endDatetime"], user_tz)
    return {"timeBlock": block}


@tool(
    category="db",
    name="get_time_entries",
    description="作業実績（タイムエントリー）を取得する。日付や期間でフィルタできる。計画との比較や振り返り時に使う。",
    input_schema={
        "properties": {
            "date": {
                "type": "string",
                "description": "特定の日付 (YYYY-MM-DD形式、省略可)",
            },
            "start_date": {
                "type": "string",
                "description": "期間の開始日 (YYYY-MM-DD形式、省略可)",
            },
            "end_date": {
                "type": "string",
                "description": "期間の終了日 (YYYY-MM-DD形式、省略可)",
            },
        },
        "required": [],
    },
)
async def get_time_entries(args: dict[str, Any]) -> dict[str, Any]:
    """Get time entries with optional date filters."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    user_tz = await get_user_timezone(user_id)

    # Convert local date filters to UTC datetime range
    start_dt = None
    end_dt = None
    target_date = parse_date(args.get("date"))
    if target_date is not None:
        start_dt, end_dt = _local_date_to_utc_range(target_date, user_tz)
    else:
        start_date = parse_date(args.get("start_date"))
        end_date = parse_date(args.get("end_date"))
        if start_date is not None and end_date is not None:
            start_dt, _ = _local_date_to_utc_range(start_date, user_tz)
            _, end_dt = _local_date_to_utc_range(end_date, user_tz)

    entries = await db_get_time_entries(
        user_id=user_id,
        start_datetime=start_dt,
        end_datetime=end_dt,
    )
    # Slim down: flatten nested objects, remove IDs not needed for reading
    # Convert UTC datetimes to user's local timezone
    return {"timeEntries": [
        {
            "startDatetime": _to_local(e["startDatetime"], user_tz),
            "endDatetime": _to_local(e["endDatetime"], user_tz),
            "taskName": e["taskName"],
            "goalName": e["goal"]["name"] if e.get("goal") else None,
            "description": e.get("description"),
        }
        for e in entries
    ]}


# =========================================================================
# Task: delete
# =========================================================================

@tool(
    category="db",
    name="delete_task",
    description="タスクを削除する。事前に get_tasks で対象タスクのIDを特定すること。",
    readonly=False,
    input_schema={
        "properties": {
            "task_id": {"type": "string", "description": "タスクID"},
        },
        "required": ["task_id"],
    },
)
async def delete_task(args: dict[str, Any]) -> dict[str, Any]:
    """Delete a task."""
    user_id = parse_uuid(args.get("user_id"))
    task_id = parse_uuid(args.get("task_id"))
    if not user_id or not task_id:
        return {"error": "Invalid or missing user_id or task_id"}
    deleted = await db_delete_task(task_id, user_id)
    return {"deleted": deleted}


# =========================================================================
# Time Block: update, delete
# =========================================================================

@tool(
    category="db",
    name="update_time_block",
    description="既存タイムブロックを更新する。事前に get_time_blocks で対象のIDを特定すること。時刻を変更する場合は date も指定すること。",
    readonly=False,
    input_schema={
        "properties": {
            "time_block_id": {"type": "string", "description": "タイムブロックID"},
            "date": {"type": "string", "description": "日付 (YYYY-MM-DD形式、時刻変更時に必要)"},
            "start_time": {"type": "string", "description": "新しい開始時刻 (HH:MM)"},
            "end_time": {"type": "string", "description": "新しい終了時刻 (HH:MM)"},
            "task_name": {"type": "string", "description": "新しい表示名"},
        },
        "required": ["time_block_id"],
    },
)
async def update_time_block(args: dict[str, Any]) -> dict[str, Any]:
    """Update an existing time block."""
    user_id = parse_uuid(args.get("user_id"))
    tb_id = parse_uuid(args.get("time_block_id"))
    if not user_id or not tb_id:
        return {"error": "Invalid or missing user_id or time_block_id"}

    user_tz = await get_user_timezone(user_id)

    # Convert local date + time to UTC datetime if provided
    target_date = parse_date(args.get("date"))
    start_time = parse_time(args.get("start_time"))
    end_time = parse_time(args.get("end_time"))

    start_dt = None
    end_dt = None
    if target_date and start_time:
        start_dt = _combine_to_utc(target_date, start_time, user_tz)
    if target_date and end_time:
        end_dt = _combine_to_utc(target_date, end_time, user_tz)
    # Handle overnight (end before start)
    if start_dt and end_dt and end_dt <= start_dt:
        end_dt += timedelta(days=1)

    block = await db_update_time_block(
        time_block_id=tb_id,
        user_id=user_id,
        start_datetime=start_dt,
        end_datetime=end_dt,
        task_name=args.get("task_name"),
    )
    if block is None:
        return {"error": "Time block not found or no updates provided"}
    # Convert UTC datetimes to user's local timezone
    if block.get("startDatetime"):
        block["startDatetime"] = _to_local(block["startDatetime"], user_tz)
    if block.get("endDatetime"):
        block["endDatetime"] = _to_local(block["endDatetime"], user_tz)
    return {"timeBlock": block}


@tool(
    category="db",
    name="delete_time_block",
    description="タイムブロックを削除する。事前に get_time_blocks で対象のIDを特定すること。task_name, start_time, end_time は表示用なので get_time_blocks の結果から必ず含めること。",
    readonly=False,
    input_schema={
        "properties": {
            "time_block_id": {"type": "string", "description": "タイムブロックID"},
            "task_name": {"type": "string", "description": "表示用: タスク名"},
            "start_time": {"type": "string", "description": "表示用: 開始時刻 (HH:MM)"},
            "end_time": {"type": "string", "description": "表示用: 終了時刻 (HH:MM)"},
        },
        "required": ["time_block_id"],
    },
)
async def delete_time_block(args: dict[str, Any]) -> dict[str, Any]:
    """Delete a time block."""
    user_id = parse_uuid(args.get("user_id"))
    tb_id = parse_uuid(args.get("time_block_id"))
    if not user_id or not tb_id:
        return {"error": "Invalid or missing user_id or time_block_id"}
    deleted = await db_delete_time_block(tb_id, user_id)
    return {"deleted": deleted}


# =========================================================================
# Memo: get, create
# =========================================================================

@tool(
    category="db",
    name="get_memos",
    description="メモ一覧を取得する。ユーザーの書いたメモや走り書きの確認に使う。",
    input_schema={
        "properties": {
            "limit": {"type": "integer", "description": "取得件数（デフォルト20）"},
        },
        "required": [],
    },
)
async def get_memos(args: dict[str, Any]) -> dict[str, Any]:
    """Get memos for a user."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}
    memos = await db_get_memos(user_id, limit=args.get("limit", 20))
    # Slim down: truncate long content
    return {"memos": [
        {**m, "content": m["content"][:300] + "..." if len(m.get("content", "")) > 300 else m.get("content", "")}
        for m in memos
    ]}


@tool(
    category="db",
    name="create_memo",
    description="メモを作成する。ユーザーが何かを書き留めたい時に使う。",
    readonly=False,
    input_schema={
        "properties": {
            "content": {"type": "string", "description": "メモの内容"},
        },
        "required": ["content"],
    },
)
async def create_memo(args: dict[str, Any]) -> dict[str, Any]:
    """Create a new memo."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}
    content = args.get("content")
    if not content:
        return {"error": "Missing content"}
    memo = await db_create_memo(user_id, content)
    return {"memo": memo}


# =========================================================================
# Note: get, create, update
# =========================================================================

@tool(
    category="db",
    name="get_notes",
    description="ノート一覧を取得する。学習記録や日記の確認に使う。日付フィルタ（start_date/end_date）はノートの内容日付（日記の対象日）で絞り込む。",
    input_schema={
        "properties": {
            "type": {"type": "string", "description": "ノート種別で絞り込み (例: diary, learning, general, book_review)"},
            "start_date": {"type": "string", "description": "開始日 (YYYY-MM-DD形式、省略可)。ノートの対象日付でフィルタ"},
            "end_date": {"type": "string", "description": "終了日 (YYYY-MM-DD形式、省略可)。ノートの対象日付でフィルタ"},
            "limit": {"type": "integer", "description": "取得件数（デフォルト20）"},
        },
        "required": [],
    },
)
async def get_notes(args: dict[str, Any]) -> dict[str, Any]:
    """Get notes for a user."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}

    user_tz = await get_user_timezone(user_id)

    notes = await db_get_notes(
        user_id,
        note_type=args.get("type"),
        start_date=parse_date(args.get("start_date")),
        end_date=parse_date(args.get("end_date")),
        limit=args.get("limit", 20),
    )
    # Slim down: truncate content, remove updatedAt
    # Convert UTC createdAt to user's local timezone
    # Include note date (content date) for clarity
    return {"notes": [
        {
            "id": n["id"],
            "title": n["title"],
            "type": n["type"],
            "date": n.get("date"),
            "content": n["content"][:300] + "..." if n.get("content") and len(n["content"]) > 300 else n.get("content"),
            "createdAt": _to_local(n.get("createdAt"), user_tz),
        }
        for n in notes
    ]}


@tool(
    category="db",
    name="create_note",
    description="ノートを作成する。学習記録や日記の新規作成に使う。",
    readonly=False,
    input_schema={
        "properties": {
            "title": {"type": "string", "description": "タイトル"},
            "content": {"type": "string", "description": "本文 (Markdown)"},
            "type": {"type": "string", "description": "ノート種別 (例: diary, learning, general, book_review)"},
        },
        "required": ["title", "content", "type"],
    },
)
async def create_note(args: dict[str, Any]) -> dict[str, Any]:
    """Create a new note."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}
    note = await db_create_note(
        user_id=user_id,
        title=args["title"],
        content=args["content"],
        note_type=args["type"],
    )
    return {"note": note}


@tool(
    category="db",
    name="update_note",
    description="既存ノートを更新する。学習記録や日記の編集に使う。事前に get_notes で対象のIDを特定すること。",
    readonly=False,
    input_schema={
        "properties": {
            "note_id": {"type": "string", "description": "ノートID"},
            "title": {"type": "string", "description": "新しいタイトル"},
            "content": {"type": "string", "description": "新しい本文"},
        },
        "required": ["note_id"],
    },
)
async def update_note(args: dict[str, Any]) -> dict[str, Any]:
    """Update an existing note."""
    user_id = parse_uuid(args.get("user_id"))
    note_id = parse_uuid(args.get("note_id"))
    if not user_id or not note_id:
        return {"error": "Invalid or missing user_id or note_id"}
    note = await db_update_note(
        note_id=note_id,
        user_id=user_id,
        title=args.get("title"),
        content=args.get("content"),
    )
    if note is None:
        return {"error": "Note not found or no updates provided"}
    return {"note": note}


# =========================================================================
# Goal: create, update, delete
# =========================================================================

@tool(
    category="db",
    name="create_goal",
    description="新しい目標を作成する。事前に get_goals_and_milestones で既存の目標を確認すること。",
    readonly=False,
    input_schema={
        "properties": {
            "name": {"type": "string", "description": "目標名"},
            "description": {"type": "string", "description": "目標の説明"},
            "color": {"type": "string", "description": "表示カラー (#hex)"},
        },
        "required": ["name"],
    },
)
async def create_goal(args: dict[str, Any]) -> dict[str, Any]:
    """Create a new goal."""
    user_id = parse_uuid(args.get("user_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}
    goal = await db_create_goal(
        user_id=user_id,
        name=args["name"],
        description=args.get("description"),
        color=args.get("color"),
    )
    return {"goal": goal}


@tool(
    category="db",
    name="update_goal",
    description="既存目標を更新する。事前に get_goals_and_milestones で対象のIDを特定すること。",
    readonly=False,
    input_schema={
        "properties": {
            "goal_id": {"type": "string", "description": "目標ID"},
            "name": {"type": "string", "description": "新しい名前"},
            "description": {"type": "string", "description": "新しい説明"},
        },
        "required": ["goal_id"],
    },
)
async def update_goal(args: dict[str, Any]) -> dict[str, Any]:
    """Update an existing goal."""
    user_id = parse_uuid(args.get("user_id"))
    goal_id = parse_uuid(args.get("goal_id"))
    if not user_id or not goal_id:
        return {"error": "Invalid or missing user_id or goal_id"}
    goal = await db_update_goal(
        goal_id=goal_id,
        user_id=user_id,
        name=args.get("name"),
        description=args.get("description"),
    )
    if goal is None:
        return {"error": "Goal not found or no updates provided"}
    return {"goal": goal}


@tool(
    category="db",
    name="delete_goal",
    description="目標を削除する。配下のマイルストーン・タスクとの紐付きに注意。事前に get_goals_and_milestones で確認すること。",
    readonly=False,
    input_schema={
        "properties": {
            "goal_id": {"type": "string", "description": "目標ID"},
        },
        "required": ["goal_id"],
    },
)
async def delete_goal(args: dict[str, Any]) -> dict[str, Any]:
    """Delete a goal."""
    user_id = parse_uuid(args.get("user_id"))
    goal_id = parse_uuid(args.get("goal_id"))
    if not user_id or not goal_id:
        return {"error": "Invalid or missing user_id or goal_id"}
    deleted = await db_delete_goal(goal_id, user_id)
    return {"deleted": deleted}


# =========================================================================
# Milestone: create, update, delete
# =========================================================================

@tool(
    category="db",
    name="create_milestone",
    description="目標にマイルストーンを追加する。",
    readonly=False,
    input_schema={
        "properties": {
            "goal_id": {"type": "string", "description": "親目標ID"},
            "name": {"type": "string", "description": "マイルストーン名"},
            "due_date": {"type": "string", "description": "期限日 (YYYY-MM-DD)"},
        },
        "required": ["goal_id", "name"],
    },
)
async def create_milestone(args: dict[str, Any]) -> dict[str, Any]:
    """Create a new milestone under a goal."""
    user_id = parse_uuid(args.get("user_id"))
    goal_id = parse_uuid(args.get("goal_id"))
    if not user_id:
        return {"error": "Invalid or missing user_id"}
    if not goal_id:
        return {"error": "Invalid or missing goal_id"}
    milestone = await db_create_milestone(
        goal_id=goal_id,
        user_id=user_id,
        name=args["name"],
        due_date=parse_date(args.get("due_date")),
    )
    return {"milestone": milestone}


@tool(
    category="db",
    name="update_milestone",
    description="既存マイルストーンを更新する。",
    readonly=False,
    input_schema={
        "properties": {
            "milestone_id": {"type": "string", "description": "マイルストーンID"},
            "name": {"type": "string", "description": "新しい名前"},
            "due_date": {"type": "string", "description": "新しい期限日"},
        },
        "required": ["milestone_id"],
    },
)
async def update_milestone(args: dict[str, Any]) -> dict[str, Any]:
    """Update an existing milestone."""
    user_id = parse_uuid(args.get("user_id"))
    milestone_id = parse_uuid(args.get("milestone_id"))
    if not user_id or not milestone_id:
        return {"error": "Invalid or missing user_id or milestone_id"}
    milestone = await db_update_milestone(
        milestone_id=milestone_id,
        user_id=user_id,
        name=args.get("name"),
        due_date=parse_date(args.get("due_date")),
    )
    if milestone is None:
        return {"error": "Milestone not found or no updates provided"}
    return {"milestone": milestone}


@tool(
    category="db",
    name="delete_milestone",
    description="マイルストーンを削除する。",
    readonly=False,
    input_schema={
        "properties": {
            "milestone_id": {"type": "string", "description": "マイルストーンID"},
        },
        "required": ["milestone_id"],
    },
)
async def delete_milestone(args: dict[str, Any]) -> dict[str, Any]:
    """Delete a milestone."""
    user_id = parse_uuid(args.get("user_id"))
    milestone_id = parse_uuid(args.get("milestone_id"))
    if not user_id or not milestone_id:
        return {"error": "Invalid or missing user_id or milestone_id"}
    deleted = await db_delete_milestone(milestone_id, user_id)
    return {"deleted": deleted}


# All DB tools for export
ALL_DB_TOOLS = [
    get_goals_and_milestones,
    get_tasks,
    create_task,
    update_task,
    delete_task,
    get_time_blocks,
    create_time_block,
    update_time_block,
    delete_time_block,
    get_time_entries,
    get_memos,
    create_memo,
    get_notes,
    create_note,
    update_note,
    create_goal,
    update_goal,
    delete_goal,
    create_milestone,
    update_milestone,
    delete_milestone,
]
