"""Database query modules."""

from kensan_ai.db.queries.goals import (
    get_goals_and_milestones,
    create_goal,
    update_goal,
    delete_goal,
    create_milestone,
    update_milestone,
    delete_milestone,
)
from kensan_ai.db.queries.tasks import get_tasks, create_task, update_task, delete_task
from kensan_ai.db.queries.time_blocks import (
    get_time_blocks,
    create_time_block,
    update_time_block,
    delete_time_block,
)
from kensan_ai.db.queries.time_entries import get_time_entries
from kensan_ai.db.queries.memos import get_memos, create_memo
from kensan_ai.db.queries.notes import get_notes, create_note, update_note
from kensan_ai.db.queries.analytics import get_analytics_summary, get_daily_summary
from kensan_ai.db.queries.patterns import get_user_patterns

__all__ = [
    "get_goals_and_milestones",
    "create_goal",
    "update_goal",
    "delete_goal",
    "create_milestone",
    "update_milestone",
    "delete_milestone",
    "get_tasks",
    "create_task",
    "update_task",
    "delete_task",
    "get_time_blocks",
    "create_time_block",
    "update_time_block",
    "delete_time_block",
    "get_time_entries",
    "get_memos",
    "create_memo",
    "get_notes",
    "create_note",
    "update_note",
    "get_analytics_summary",
    "get_daily_summary",
    "get_user_patterns",
]
