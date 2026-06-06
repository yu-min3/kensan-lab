"""Pattern queries for behavior analysis and planning recommendations."""

from datetime import date, datetime, timedelta
from typing import Any
from uuid import UUID
from zoneinfo import ZoneInfo

from kensan_ai.db.connection import get_connection


async def get_user_patterns(
    user_id: UUID,
    lookback_weeks: int,
    timezone: ZoneInfo,
) -> dict[str, Any]:
    """Compute user behavior patterns for planning recommendations.

    Args:
        user_id: The user ID.
        lookback_weeks: Number of weeks to look back (max 12).
        timezone: User's timezone for local time calculations.

    Returns:
        Dict with productivityByHour, planAccuracy, overcommitRatio,
        chronicOverdueTasks, goalVelocity, avgSessionMinutes.
    """
    lookback_weeks = min(lookback_weeks, 12)
    tz_name = str(timezone)

    today = datetime.now(timezone)
    start_local = today.replace(hour=0, minute=0, second=0, microsecond=0) - timedelta(weeks=lookback_weeks)
    end_local = today.replace(hour=0, minute=0, second=0, microsecond=0) + timedelta(days=1)
    start_utc = start_local.astimezone(ZoneInfo("UTC"))
    end_utc = end_local.astimezone(ZoneInfo("UTC"))

    async with get_connection() as conn:
        # 1. Productivity by hour (actual time entries)
        actual_by_hour = await conn.fetch(
            """
            SELECT
                EXTRACT(HOUR FROM (start_datetime AT TIME ZONE $4)) AS hour,
                SUM(EXTRACT(EPOCH FROM (end_datetime - start_datetime)) / 60)::integer AS total_minutes,
                COUNT(*) AS entry_count
            FROM time_entries
            WHERE user_id = $1
              AND start_datetime >= $2
              AND start_datetime < $3
            GROUP BY EXTRACT(HOUR FROM (start_datetime AT TIME ZONE $4))
            ORDER BY hour
            """,
            user_id,
            start_utc,
            end_utc,
            tz_name,
        )

        # 2. Planned by hour (time blocks)
        planned_by_hour = await conn.fetch(
            """
            SELECT
                EXTRACT(HOUR FROM (start_datetime AT TIME ZONE $4)) AS hour,
                SUM(EXTRACT(EPOCH FROM (end_datetime - start_datetime)) / 60)::integer AS total_minutes,
                COUNT(*) AS block_count
            FROM time_blocks
            WHERE user_id = $1
              AND start_datetime >= $2
              AND start_datetime < $3
            GROUP BY EXTRACT(HOUR FROM (start_datetime AT TIME ZONE $4))
            ORDER BY hour
            """,
            user_id,
            start_utc,
            end_utc,
            tz_name,
        )

        # 3. Total planned vs actual (for plan accuracy / overcommit ratio)
        totals = await conn.fetchrow(
            """
            SELECT
                (SELECT COALESCE(SUM(EXTRACT(EPOCH FROM (end_datetime - start_datetime)) / 60), 0)
                 FROM time_blocks WHERE user_id = $1 AND start_datetime >= $2 AND start_datetime < $3
                ) AS planned_minutes,
                (SELECT COALESCE(SUM(EXTRACT(EPOCH FROM (end_datetime - start_datetime)) / 60), 0)
                 FROM time_entries WHERE user_id = $1 AND start_datetime >= $2 AND start_datetime < $3
                ) AS actual_minutes
            """,
            user_id,
            start_utc,
            end_utc,
        )

        # 4. Chronic overdue tasks (incomplete, overdue, created > 2 weeks ago)
        overdue_tasks = await conn.fetch(
            """
            SELECT
                t.id::text AS task_id,
                t.name AS task_name,
                COALESCE(g.name, '') AS goal_name,
                t.due_date::text AS due_date,
                (CURRENT_DATE - t.due_date::date) AS days_overdue
            FROM tasks t
            LEFT JOIN milestones m ON t.milestone_id = m.id
            LEFT JOIN goals g ON m.goal_id = g.id
            WHERE t.user_id = $1
              AND t.completed = false
              AND t.due_date IS NOT NULL
              AND t.due_date < CURRENT_DATE
              AND t.created_at < NOW() - INTERVAL '2 weeks'
            ORDER BY days_overdue DESC
            """,
            user_id,
        )

        # 5. Goal weekly minutes (for velocity / trend)
        goal_weekly = await conn.fetch(
            """
            SELECT
                goal_id::text,
                goal_name,
                goal_color,
                date_trunc('week', (start_datetime AT TIME ZONE $4))::date AS week_start,
                SUM(EXTRACT(EPOCH FROM (end_datetime - start_datetime)) / 60)::integer AS minutes
            FROM time_entries
            WHERE user_id = $1
              AND start_datetime >= $2
              AND start_datetime < $3
              AND goal_id IS NOT NULL
            GROUP BY goal_id, goal_name, goal_color, week_start
            ORDER BY goal_id, week_start
            """,
            user_id,
            start_utc,
            end_utc,
            tz_name,
        )

        # 6. Average session duration
        avg_session = await conn.fetchrow(
            """
            SELECT
                COALESCE(AVG(EXTRACT(EPOCH FROM (end_datetime - start_datetime)) / 60), 0)::integer AS avg_minutes
            FROM time_entries
            WHERE user_id = $1
              AND start_datetime >= $2
              AND start_datetime < $3
            """,
            user_id,
            start_utc,
            end_utc,
        )

    # Post-processing

    # Build planned-by-hour lookup
    planned_lookup: dict[int, int] = {}
    for row in planned_by_hour:
        planned_lookup[int(row["hour"])] = int(row["total_minutes"] or 0)

    # Productivity by hour with execution rate
    productivity_by_hour = []
    for row in actual_by_hour:
        hour = int(row["hour"])
        actual_min = int(row["total_minutes"] or 0)
        planned_min = planned_lookup.get(hour, 0)
        exec_rate = round(actual_min / planned_min, 2) if planned_min > 0 else None
        productivity_by_hour.append({
            "hour": hour,
            "totalMinutes": actual_min,
            "entryCount": int(row["entry_count"]),
            "executionRate": exec_rate,
        })

    # Plan accuracy & overcommit ratio
    planned_total = float(totals["planned_minutes"]) if totals else 0
    actual_total = float(totals["actual_minutes"]) if totals else 0
    plan_accuracy = round(actual_total / planned_total, 2) if planned_total > 0 else None
    overcommit_ratio = round(planned_total / actual_total, 2) if actual_total > 0 else None

    # Chronic overdue tasks
    chronic_overdue = [
        {
            "taskId": row["task_id"],
            "taskName": row["task_name"],
            "goalName": row["goal_name"],
            "daysOverdue": int(row["days_overdue"]),
        }
        for row in overdue_tasks
    ]

    # Goal velocity with trend calculation
    goal_data: dict[str, dict[str, Any]] = {}
    for row in goal_weekly:
        gid = row["goal_id"]
        if gid not in goal_data:
            goal_data[gid] = {
                "goalId": gid,
                "goalName": row["goal_name"] or "未分類",
                "goalColor": row["goal_color"] or "#888888",
                "weeklyMinutes": [],
            }
        goal_data[gid]["weeklyMinutes"].append(int(row["minutes"] or 0))

    goal_velocity = []
    for gd in goal_data.values():
        weekly = gd["weeklyMinutes"]
        trend, change_pct = _calculate_trend(weekly)
        goal_velocity.append({
            "goalId": gd["goalId"],
            "goalName": gd["goalName"],
            "goalColor": gd["goalColor"],
            "weeklyMinutes": weekly,
            "trend": trend,
            "changePercent": change_pct,
        })

    return {
        "productivityByHour": productivity_by_hour,
        "planAccuracy": plan_accuracy,
        "overcommitRatio": overcommit_ratio,
        "chronicOverdueTasks": chronic_overdue,
        "goalVelocity": goal_velocity,
        "avgSessionMinutes": int(avg_session["avg_minutes"]) if avg_session else 0,
        "lookbackWeeks": lookback_weeks,
    }


def _calculate_trend(weekly_minutes: list[int]) -> tuple[str, float]:
    """Calculate trend from weekly minutes data.

    Returns:
        Tuple of (trend_label, change_percent).
        trend_label: 'accelerating' | 'stable' | 'declining' | 'stalled'
    """
    if len(weekly_minutes) < 2:
        return ("stable", 0.0)

    # Compare last 2 weeks vs the prior weeks
    recent = weekly_minutes[-2:]
    recent_avg = sum(recent) / len(recent)

    if len(weekly_minutes) >= 4:
        prior = weekly_minutes[:-2]
    else:
        prior = weekly_minutes[:1]
    prior_avg = sum(prior) / len(prior)

    if prior_avg == 0:
        if recent_avg == 0:
            return ("stalled", 0.0)
        return ("accelerating", 100.0)

    change = ((recent_avg - prior_avg) / prior_avg) * 100
    change = round(change, 1)

    if recent_avg == 0:
        return ("stalled", change)
    elif change > 15:
        return ("accelerating", change)
    elif change < -15:
        return ("declining", change)
    else:
        return ("stable", change)
