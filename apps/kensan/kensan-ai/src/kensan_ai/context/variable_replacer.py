"""Variable replacer for dynamic system prompt substitution."""

import asyncio
import json
import re
from collections.abc import Coroutine
from datetime import date, datetime, timedelta
from typing import Any
from uuid import UUID
from zoneinfo import ZoneInfo

from kensan_ai.db.connection import get_connection
from kensan_ai.db.queries.patterns import get_user_patterns as db_get_user_patterns
from kensan_ai.db.queries.user_settings import get_user_timezone
from kensan_ai.lib.timezone_utils import DEFAULT_TZ as _DEFAULT_TZ, local_date_to_utc_range, local_today


class VariableReplacer:
    """Replaces variables in system prompts with dynamic content."""

    # Pattern to match variables like {user_memory}, {today_schedule}, etc.
    VARIABLE_PATTERN = re.compile(r"\{(\w+)\}")

    # Supported variables
    SUPPORTED_VARIABLES = {
        "current_datetime",
        "user_memory",
        "today_schedule",
        "tomorrow_schedule",
        "today_entries",
        "pending_tasks",
        "recent_context",
        "weekly_summary",
        "goal_progress",
        "user_patterns",
        "emotion_summary",
        "interest_profile",
        "user_traits",
        "communication_style",
        "yesterday_entries",
        "recent_learning_notes",
    }

    # Variable metadata: name → {description, example}
    VARIABLE_METADATA: dict[str, dict[str, str]] = {
        "current_datetime": {
            "description": "現在の日時（JST）。曜日付き。",
            "example": "2026-02-08（日）14:30 JST",
        },
        "user_memory": {
            "description": "ユーザーのプロフィール概要・強み・成長領域。user_memory テーブルから取得。",
            "example": "バックエンドエンジニア。強み: Go, 設計力。成長領域: フロントエンド",
        },
        "today_schedule": {
            "description": "今日のタイムブロック一覧（予定）。時間順。",
            "example": "- 09:00〜10:30: API設計 [Kensanプロジェクト]\n- 14:00〜16:00: コードレビュー",
        },
        "tomorrow_schedule": {
            "description": "明日のタイムブロック一覧（予定）。時間順。",
            "example": "- 10:00〜12:00: DB設計 [Kensanプロジェクト]",
        },
        "today_entries": {
            "description": "今日の実績タイムエントリ一覧。実際に記録された作業時間。",
            "example": "- 09:15〜10:45: API実装 [Kensanプロジェクト]",
        },
        "pending_tasks": {
            "description": "未完了タスク一覧。期限順、目標付き。期限超過には警告。",
            "example": "- DB設計書作成 (⚠️ 明日期限, Kensanプロジェクト)\n- テスト追加 (あと5日)",
        },
        "recent_context": {
            "description": "直近のAI会話履歴（最新3件）。文脈の継続に使用。",
            "example": "[02/07 15:30 - chat]\nユーザー: 来週の予定を確認して\nAI: 来週は3件のブロックが...",
        },
        "weekly_summary": {
            "description": "今週の稼働サマリー。目標別の作業時間集計。",
            "example": "期間: 2026-02-03 〜 2026-02-09\n総稼働: 32.5h\n目標別:\n- Kensan: 20.0h",
        },
        "goal_progress": {
            "description": "アクティブな目標とマイルストーンの進捗。タスク完了率付き。",
            "example": "【Kensanプロジェクト】\n  ○ API実装 (3/5)\n  ✓ DB設計 (4/4)",
        },
        "user_patterns": {
            "description": "ユーザーの行動パターン分析。計画精度、生産性ピーク時間帯、目標トレンド。",
            "example": "計画精度: 78%\n生産性ピーク時間帯: 10時(120分), 14時(90分)\n目標「Kensan」: 加速中(+15%)",
        },
        "emotion_summary": {
            "description": "Lakehouse Gold層の感情分析サマリー。週次の感情・エネルギー・ストレス。",
            "example": "今週の感情: ポジティブ / エネルギー: 高い / ストレス: 普通\n主な感情: 充実感",
        },
        "interest_profile": {
            "description": "Lakehouse Gold層の関心プロファイル。タグ使用頻度、トレンド、クラスタ。",
            "example": "関心タグ: Go(15件)↑, React(12件), DB設計(8件)\n最近伸びている関心: Rust, WebAssembly",
        },
        "user_traits": {
            "description": "Lakehouse Gold層のユーザー特性プロファイル。仕事・学習スタイル、強み・課題。",
            "example": "仕事スタイル: deep_focus\n学習スタイル: hands-on\n強み: 設計力, コードレビュー",
        },
        "communication_style": {
            "description": "関心・特性データから導出されたコミュニケーションスタイルガイド。AIの応答トーン指示。",
            "example": "技術的な詳細を省略しない / 集中を妨げない簡潔な回答 / 具体的なコード例を提示",
        },
        "yesterday_entries": {
            "description": "昨日のタイムエントリ実績一覧。昨日実際に記録された作業時間。",
            "example": "- 19:00〜20:00: ACE試験対策 [GCPスキルアップ]\n合計: 2.5h (3セッション)",
        },
        "recent_learning_notes": {
            "description": "直近3日間の学習記録・日記のタイトルと概要。",
            "example": "【02/07 learning】GCP IAMまとめ [GCPスキルアップ] tags: GCP, IAM",
        },
    }

    @staticmethod
    def get_variable_metadata() -> list[dict[str, Any]]:
        """Get metadata for all supported variables.

        Returns a list of dicts with name, description, example, and excludes_tools.
        Merges with VARIABLE_EXCLUDES_TOOLS from chat.py.
        """
        from kensan_ai.agents.chat import VARIABLE_EXCLUDES_TOOLS

        result = []
        for name, meta in VariableReplacer.VARIABLE_METADATA.items():
            result.append({
                "name": name,
                "description": meta["description"],
                "example": meta["example"],
                "excludes_tools": VARIABLE_EXCLUDES_TOOLS.get(name, []),
            })
        return result

    @staticmethod
    async def replace(system_prompt: str, user_id: UUID) -> str:
        """Replace variables in the system prompt with dynamic content.

        Args:
            system_prompt: The system prompt with potential variables
            user_id: The user ID to fetch data for

        Returns:
            The system prompt with variables replaced
        """
        # Find all variables in the prompt
        variables = VariableReplacer.VARIABLE_PATTERN.findall(system_prompt)

        # Filter to only supported variables
        variables_to_replace = [v for v in variables if v in VariableReplacer.SUPPORTED_VARIABLES]

        if not variables_to_replace:
            return system_prompt

        # Fetch all needed data
        replacements: dict[str, str] = {}

        # Async variable name → factory (callable that returns a coroutine)
        _ASYNC_FACTORIES = {
            "user_memory": VariableReplacer._get_user_memory,
            "today_schedule": VariableReplacer._get_today_schedule,
            "tomorrow_schedule": VariableReplacer._get_tomorrow_schedule,
            "pending_tasks": VariableReplacer._get_pending_tasks,
            "today_entries": VariableReplacer._get_today_entries,
            "recent_context": VariableReplacer._get_recent_context,
            "weekly_summary": VariableReplacer._get_weekly_summary,
            "goal_progress": VariableReplacer._get_goal_progress,
            "user_patterns": VariableReplacer._get_user_patterns,
            "yesterday_entries": VariableReplacer._get_yesterday_entries,
            "recent_learning_notes": VariableReplacer._get_recent_learning_notes,
        }

        # Sync variable name → callable (lakehouse-based, no await)
        _SYNC_FETCHERS = {
            "emotion_summary": VariableReplacer._get_emotion_summary,
            "interest_profile": VariableReplacer._get_interest_profile,
            "user_traits": VariableReplacer._get_user_traits,
            "communication_style": VariableReplacer._get_communication_style,
        }

        # Process sync variables (current_datetime + lakehouse)
        for var in variables_to_replace:
            if var == "current_datetime":
                now = datetime.now(_DEFAULT_TZ)
                weekday_names = ["月", "火", "水", "木", "金", "土", "日"]
                weekday = weekday_names[now.weekday()]
                replacements[var] = f"{now.strftime('%Y-%m-%d')}（{weekday}）{now.strftime('%H:%M')} JST"
            elif var in _SYNC_FETCHERS:
                replacements[var] = _SYNC_FETCHERS[var](user_id)

        # Collect async tasks only for variables that are actually needed
        async_tasks: dict[str, Coroutine] = {}
        for var in variables_to_replace:
            if var in _ASYNC_FACTORIES:
                async_tasks[var] = _ASYNC_FACTORIES[var](user_id)

        # Execute all async fetchers in parallel
        if async_tasks:
            results = await asyncio.gather(*async_tasks.values())
            for key, result in zip(async_tasks.keys(), results):
                replacements[key] = result

        # Replace variables in the prompt
        result = system_prompt
        for var, value in replacements.items():
            result = result.replace(f"{{{var}}}", value)

        return result

    @staticmethod
    async def _get_user_memory(user_id: UUID) -> str:
        """Get user's profile summary from user_memory table."""
        async with get_connection() as conn:
            row = await conn.fetchrow(
                """
                SELECT profile_summary, preferences, strengths, growth_areas
                FROM user_memory
                WHERE user_id = $1
                """,
                user_id,
            )

            if not row or not row["profile_summary"]:
                return "（ユーザー情報なし）"

            parts = [row["profile_summary"]]

            if row["strengths"]:
                strengths = list(row["strengths"])
                if strengths:
                    parts.append(f"強み: {', '.join(strengths)}")

            if row["growth_areas"]:
                growth_areas = list(row["growth_areas"])
                if growth_areas:
                    parts.append(f"成長領域: {', '.join(growth_areas)}")

            return "\n".join(parts)

    @staticmethod
    async def _get_today_schedule(user_id: UUID) -> str:
        """Get today's time blocks."""
        today = local_today()
        start_utc, end_utc = local_date_to_utc_range(today)

        async with get_connection() as conn:
            rows = await conn.fetch(
                """
                SELECT start_datetime, end_datetime, task_name, goal_name
                FROM time_blocks
                WHERE user_id = $1
                  AND start_datetime >= $2
                  AND start_datetime < $3
                ORDER BY start_datetime
                """,
                user_id,
                start_utc,
                end_utc,
            )

            if not rows:
                return "（今日の予定なし）"

            schedule_items = []
            for row in rows:
                start = row["start_datetime"].astimezone(_DEFAULT_TZ).strftime("%H:%M")
                end = row["end_datetime"].astimezone(_DEFAULT_TZ).strftime("%H:%M")
                task = row["task_name"]
                goal = f" [{row['goal_name']}]" if row["goal_name"] else ""
                schedule_items.append(f"- {start}〜{end}: {task}{goal}")

            return "\n".join(schedule_items)

    @staticmethod
    async def _get_tomorrow_schedule(user_id: UUID) -> str:
        """Get tomorrow's time blocks."""
        tomorrow = local_today() + timedelta(days=1)
        start_utc, end_utc = local_date_to_utc_range(tomorrow)

        async with get_connection() as conn:
            rows = await conn.fetch(
                """
                SELECT start_datetime, end_datetime, task_name, goal_name
                FROM time_blocks
                WHERE user_id = $1
                  AND start_datetime >= $2
                  AND start_datetime < $3
                ORDER BY start_datetime
                """,
                user_id,
                start_utc,
                end_utc,
            )

            if not rows:
                return "（明日の予定なし）"

            schedule_items = []
            for row in rows:
                start = row["start_datetime"].astimezone(_DEFAULT_TZ).strftime("%H:%M")
                end = row["end_datetime"].astimezone(_DEFAULT_TZ).strftime("%H:%M")
                task = row["task_name"]
                goal = f" [{row['goal_name']}]" if row["goal_name"] else ""
                schedule_items.append(f"- {start}〜{end}: {task}{goal}")

            return "\n".join(schedule_items)

    @staticmethod
    async def _get_pending_tasks(user_id: UUID, limit: int = 10) -> str:
        """Get pending (incomplete) tasks, prioritized by due date.

        Non-routine tasks only. Due dates shown as "あとN日" for clarity.
        """
        today = local_today()
        async with get_connection() as conn:
            rows = await conn.fetch(
                """
                SELECT t.name, t.due_date,
                       m.name AS milestone_name,
                       g.name AS goal_name
                FROM tasks t
                LEFT JOIN milestones m ON t.milestone_id = m.id
                LEFT JOIN goals g ON m.goal_id = g.id
                WHERE t.user_id = $1
                  AND t.completed = false
                  AND t.frequency IS NULL
                ORDER BY
                    CASE WHEN t.due_date IS NULL THEN 1 ELSE 0 END,
                    t.due_date ASC
                LIMIT $2
                """,
                user_id,
                limit,
            )

            if not rows:
                return "（未完了タスクなし）"

            task_items = []
            for row in rows:
                task = row["name"]
                parts = []

                if row["due_date"]:
                    days_remaining = (row["due_date"] - today).days
                    if days_remaining < 0:
                        parts.append(f"⚠️ {abs(days_remaining)}日超過")
                    elif days_remaining == 0:
                        parts.append("⚠️ 今日期限")
                    elif days_remaining == 1:
                        parts.append("⚠️ 明日期限")
                    elif days_remaining <= 7:
                        parts.append(f"あと{days_remaining}日")
                    else:
                        parts.append(f"あと{days_remaining}日（余裕あり）")
                if row["goal_name"]:
                    parts.append(row["goal_name"])

                suffix = f" ({', '.join(parts)})" if parts else ""
                task_items.append(f"- {task}{suffix}")

            return "\n".join(task_items)

    @staticmethod
    async def _get_today_entries(user_id: UUID) -> str:
        """Get today's actual time entries."""
        today = local_today()
        start_utc, end_utc = local_date_to_utc_range(today)

        async with get_connection() as conn:
            rows = await conn.fetch(
                """
                SELECT start_datetime, end_datetime, task_name, goal_name
                FROM time_entries
                WHERE user_id = $1
                  AND start_datetime >= $2
                  AND start_datetime < $3
                ORDER BY start_datetime
                """,
                user_id,
                start_utc,
                end_utc,
            )

            if not rows:
                return "（今日の実績なし）"

            entry_items = []
            for row in rows:
                start = row["start_datetime"].astimezone(_DEFAULT_TZ).strftime("%H:%M")
                end = row["end_datetime"].astimezone(_DEFAULT_TZ).strftime("%H:%M")
                task = row["task_name"]
                goal = f" [{row['goal_name']}]" if row["goal_name"] else ""
                entry_items.append(f"- {start}〜{end}: {task}{goal}")

            return "\n".join(entry_items)

    @staticmethod
    async def _get_weekly_summary(user_id: UUID) -> str:
        """Get this week's analytics summary (Mon-Sun)."""
        today = local_today()
        # Monday of this week
        week_start = today - timedelta(days=today.weekday())
        # Sunday of this week
        week_end = week_start + timedelta(days=6)

        # Convert local dates to UTC range
        start_utc, _ = local_date_to_utc_range(week_start)
        _, end_utc = local_date_to_utc_range(week_end)

        async with get_connection() as conn:
            rows = await conn.fetch(
                """
                SELECT
                    goal_name,
                    SUM(EXTRACT(EPOCH FROM (end_datetime - start_datetime)) / 3600) as total_hours
                FROM time_entries
                WHERE user_id = $1
                  AND start_datetime >= $2
                  AND start_datetime < $3
                GROUP BY goal_name
                ORDER BY total_hours DESC
                """,
                user_id,
                start_utc,
                end_utc,
            )

            if not rows:
                return "（今週の実績なし）"

            total = 0.0
            breakdown = []
            for row in rows:
                hours = float(row["total_hours"] or 0)
                total += hours
                goal = row["goal_name"] or "未分類"
                breakdown.append(f"- {goal}: {hours:.1f}h")

            lines = [
                f"期間: {week_start.isoformat()} 〜 {week_end.isoformat()}",
                f"総稼働: {total:.1f}h",
                "目標別:",
                *breakdown,
            ]
            return "\n".join(lines)

    @staticmethod
    async def _get_goal_progress(user_id: UUID) -> str:
        """Get goal and milestone progress summary."""
        async with get_connection() as conn:
            goals = await conn.fetch(
                """
                SELECT id, name
                FROM goals
                WHERE user_id = $1 AND status != 'archived'
                ORDER BY created_at
                """,
                user_id,
            )

            if not goals:
                return "（目標なし）"

            goal_items = []
            for goal in goals:
                milestones = await conn.fetch(
                    """
                    SELECT m.name, m.status, m.target_date,
                           COUNT(t.id) as total_tasks,
                           COUNT(t.id) FILTER (WHERE t.completed = true) as done_tasks
                    FROM milestones m
                    LEFT JOIN tasks t ON t.milestone_id = m.id
                    WHERE m.goal_id = $1 AND m.status != 'archived'
                    GROUP BY m.id, m.name, m.status, m.target_date
                    ORDER BY m.target_date NULLS LAST
                    """,
                    goal["id"],
                )

                ms_lines = []
                for ms in milestones:
                    total = ms["total_tasks"]
                    done = ms["done_tasks"]
                    pct = f" ({done}/{total})" if total > 0 else ""
                    due = f" 期限:{ms['target_date'].isoformat()}" if ms["target_date"] else ""
                    status = "✓" if ms["status"] == "completed" else "○"
                    ms_lines.append(f"  {status} {ms['name']}{pct}{due}")

                goal_items.append(f"【{goal['name']}】")
                if ms_lines:
                    goal_items.extend(ms_lines)
                else:
                    goal_items.append("  （マイルストーンなし）")

            return "\n".join(goal_items)

    @staticmethod
    async def _get_user_patterns(user_id: UUID) -> str:
        """Get user behavior patterns as formatted text."""
        user_tz = await get_user_timezone(user_id)
        patterns = await db_get_user_patterns(user_id, lookback_weeks=4, timezone=user_tz)

        lines = []

        # Plan accuracy
        if patterns.get("planAccuracy") is not None:
            lines.append(f"計画精度: {patterns['planAccuracy']:.0%}")
        if patterns.get("overcommitRatio") is not None:
            ratio = patterns["overcommitRatio"]
            label = "計画過多" if ratio > 1.1 else "計画不足" if ratio < 0.9 else "適正"
            lines.append(f"計画/実績比: {ratio:.2f}（{label}）")

        # Average session
        if patterns.get("avgSessionMinutes"):
            lines.append(f"平均作業セッション: {patterns['avgSessionMinutes']}分")

        # Productivity by hour
        prod = patterns.get("productivityByHour", [])
        if prod:
            sorted_prod = sorted(prod, key=lambda x: x["totalMinutes"], reverse=True)
            top3 = sorted_prod[:3]
            peak_str = ", ".join(f"{p['hour']}時({p['totalMinutes']}分)" for p in top3)
            lines.append(f"生産性ピーク時間帯: {peak_str}")

        # Goal velocity
        for gv in patterns.get("goalVelocity", []):
            trend_labels = {
                "accelerating": "加速中",
                "stable": "安定",
                "declining": "減速中",
                "stalled": "停滞",
            }
            trend = trend_labels.get(gv["trend"], gv["trend"])
            lines.append(f"目標「{gv['goalName']}」: {trend}（{gv['changePercent']:+.0f}%）")

        # Chronic overdue
        overdue = patterns.get("chronicOverdueTasks", [])
        if overdue:
            lines.append(f"慢性繰り越しタスク（{len(overdue)}件）:")
            for t in overdue[:5]:
                lines.append(f"  - {t['taskName']}（{t['daysOverdue']}日超過）")

        return "\n".join(lines) if lines else "（行動パターンデータなし）"

    @staticmethod
    def _get_emotion_summary(user_id: UUID) -> str:
        """Get emotion summary from Lakehouse Gold layer."""
        try:
            from kensan_ai.lakehouse.reader import get_reader

            reader = get_reader()
            weeks = reader.get_emotion_weekly(str(user_id), weeks=4)
        except Exception:
            return "（感情データなし）"

        if not weeks:
            return "（感情データなし）"

        # 最新週のデータ
        latest = weeks[0]

        valence = latest.get("avg_valence")
        energy = latest.get("avg_energy")
        stress = latest.get("avg_stress")
        emotion = latest.get("dominant_emotion") or "不明"
        trend = latest.get("valence_trend") or "stable"
        diary_count = latest.get("diary_count") or 0

        # valenceラベル
        if valence is not None:
            if valence > 0.3:
                v_label = "ポジティブ"
            elif valence < -0.3:
                v_label = "ネガティブ"
            else:
                v_label = "中立"
        else:
            v_label = "不明"

        # energyラベル
        if energy is not None:
            e_label = "高い" if energy > 0.6 else "低い" if energy < 0.4 else "普通"
        else:
            e_label = "不明"

        # stressラベル
        if stress is not None:
            s_label = "高い" if stress > 0.6 else "低い" if stress < 0.4 else "普通"
        else:
            s_label = "不明"

        # trendラベル
        trend_labels = {"improving": "改善傾向", "stable": "安定", "declining": "低下傾向"}
        trend_label = trend_labels.get(trend, "安定")

        lines = [
            f"今週の感情: {v_label} / エネルギー: {e_label} / ストレス: {s_label}",
            f"主な感情: {emotion}",
            f"傾向: {trend_label}（日記{diary_count}件から分析）",
        ]

        # タスク相関
        task_corr_json = latest.get("task_correlation_json")
        if task_corr_json:
            try:
                import json

                correlations = json.loads(task_corr_json)
                if correlations:
                    lines.append("タスクと感情の相関:")
                    for tc in sorted(correlations, key=lambda x: x.get("avg_valence", 0), reverse=True):
                        sign = "+" if tc.get("avg_valence", 0) >= 0 else "-"
                        lines.append(f"  {sign} {tc['task_name']}: {tc['avg_valence']}")
            except (json.JSONDecodeError, TypeError):
                pass

        return "\n".join(lines)

    @staticmethod
    def _get_interest_profile(user_id: UUID) -> str:
        """Get user interest profile from Lakehouse Gold layer."""
        try:
            from kensan_ai.lakehouse.reader import get_reader

            reader = get_reader()
            profile = reader.get_interest_profile(str(user_id))
        except Exception:
            return "（関心データなし）"

        if not profile:
            return "（関心データなし）"

        lines = []

        # Top tags
        top_tags_json = profile.get("top_tags_json")
        if top_tags_json:
            try:
                top_tags = json.loads(top_tags_json)
                if top_tags:
                    tag_strs = []
                    for t in top_tags[:10]:
                        name = t.get("name", "")
                        count = t.get("count", 0)
                        trend = t.get("trend", "stable")
                        trend_mark = {"growing": "↑", "fading": "↓"}.get(trend, "")
                        tag_strs.append(f"{name}({count}件){trend_mark}")
                    lines.append(f"関心タグ: {', '.join(tag_strs)}")
            except (json.JSONDecodeError, TypeError):
                pass

        # Emerging tags
        emerging_json = profile.get("emerging_tags_json")
        if emerging_json:
            try:
                emerging = json.loads(emerging_json)
                if emerging:
                    names = [t.get("name", "") for t in emerging[:5]]
                    lines.append(f"最近伸びている関心: {', '.join(names)}")
            except (json.JSONDecodeError, TypeError):
                pass

        # Fading tags
        fading_json = profile.get("fading_tags_json")
        if fading_json:
            try:
                fading = json.loads(fading_json)
                if fading:
                    names = [t.get("name", "") for t in fading[:5]]
                    lines.append(f"最近減っている関心: {', '.join(names)}")
            except (json.JSONDecodeError, TypeError):
                pass

        # Clusters
        clusters_json = profile.get("tag_clusters_json")
        if clusters_json:
            try:
                clusters = json.loads(clusters_json)
                if clusters:
                    cluster_strs = [" + ".join(c) for c in clusters[:5]]
                    lines.append(f"関心クラスタ: {' / '.join(cluster_strs)}")
            except (json.JSONDecodeError, TypeError):
                pass

        total = profile.get("total_tagged_notes", 0)
        if total:
            lines.append(f"タグ付きノート総数: {total}件")

        return "\n".join(lines) if lines else "（関心データなし）"

    @staticmethod
    def _get_user_traits(user_id: UUID) -> str:
        """Get user trait profile from Lakehouse Gold layer."""
        try:
            from kensan_ai.lakehouse.reader import get_reader

            reader = get_reader()
            profile = reader.get_trait_profile(str(user_id))
        except Exception:
            return "（特性データなし）"

        if not profile:
            return "（特性データなし）"

        lines = []

        if profile.get("work_style"):
            lines.append(f"仕事スタイル: {profile['work_style']}")
        if profile.get("learning_style"):
            lines.append(f"学習スタイル: {profile['learning_style']}")
        if profile.get("collaboration"):
            lines.append(f"協業スタイル: {profile['collaboration']}")

        # Strengths
        strengths_json = profile.get("strengths_json")
        if strengths_json:
            try:
                strengths = json.loads(strengths_json)
                if strengths:
                    lines.append(f"強み: {', '.join(strengths[:5])}")
            except (json.JSONDecodeError, TypeError):
                pass

        # Challenges
        challenges_json = profile.get("challenges_json")
        if challenges_json:
            try:
                challenges = json.loads(challenges_json)
                if challenges:
                    lines.append(f"課題: {', '.join(challenges[:5])}")
            except (json.JSONDecodeError, TypeError):
                pass

        # Triggers
        triggers_json = profile.get("triggers_json")
        if triggers_json:
            try:
                triggers = json.loads(triggers_json)
                if triggers:
                    lines.append(f"モチベーショントリガー: {', '.join(triggers[:5])}")
            except (json.JSONDecodeError, TypeError):
                pass

        return "\n".join(lines) if lines else "（特性データなし）"

    @staticmethod
    def _get_communication_style(user_id: UUID) -> str:
        """Derive communication style guidance from interest + trait profiles.

        Combines tag-based interest data and personality traits to generate
        concise instructions for how the AI should communicate with this user.
        Returns empty string when data is insufficient (no prompt impact).
        """
        try:
            from kensan_ai.lakehouse.reader import get_reader

            reader = get_reader()
            interest = reader.get_interest_profile(str(user_id))
            traits = reader.get_trait_profile(str(user_id))
        except Exception:
            return ""

        if not interest and not traits:
            return ""

        guidelines: list[str] = []

        # Technical depth from interest profile
        if interest:
            top_tags_json = interest.get("top_tags_json")
            if top_tags_json:
                try:
                    top_tags = json.loads(top_tags_json)
                    tech_count = sum(
                        1 for t in top_tags
                        if any(kw in t.get("name", "").lower()
                               for kw in ("python", "go", "rust", "react", "typescript",
                                          "docker", "k8s", "api", "db", "sql", "git",
                                          "aws", "gcp", "linux", "ci", "infra"))
                    )
                    if len(top_tags) > 0:
                        tech_ratio = tech_count / len(top_tags)
                        if tech_ratio > 0.5:
                            guidelines.append("技術的な詳細を省略しない")
                        elif tech_ratio < 0.2:
                            guidelines.append("技術用語は平易に言い換える")
                except (json.JSONDecodeError, TypeError):
                    pass

        # Style from trait profile
        if traits:
            work_style = traits.get("work_style", "")
            if work_style:
                style_map = {
                    "methodical": "ステップバイステップで整理して提案",
                    "spontaneous": "柔軟な選択肢を提示",
                    "deep_focus": "集中を妨げない簡潔な回答",
                    "multitasker": "要点を箇条書きで整理",
                }
                for key, advice in style_map.items():
                    if key in work_style.lower():
                        guidelines.append(advice)
                        break

            learning_style = traits.get("learning_style", "")
            if learning_style:
                if "visual" in learning_style.lower():
                    guidelines.append("図や構造化された表現を活用")
                elif "hands-on" in learning_style.lower() or "practical" in learning_style.lower():
                    guidelines.append("具体的なコード例や手順を提示")

            # Challenges → encouragement style
            challenges_json = traits.get("challenges_json")
            if challenges_json:
                try:
                    challenges = json.loads(challenges_json)
                    if challenges:
                        guidelines.append("小さなステップに分割して提案")
                except (json.JSONDecodeError, TypeError):
                    pass

        return " / ".join(guidelines) if guidelines else ""

    @staticmethod
    async def _get_yesterday_entries(user_id: UUID) -> str:
        """Get yesterday's actual time entries."""
        yesterday = local_today() - timedelta(days=1)
        start_utc, end_utc = local_date_to_utc_range(yesterday)

        async with get_connection() as conn:
            rows = await conn.fetch(
                """
                SELECT start_datetime, end_datetime, task_name, goal_name
                FROM time_entries
                WHERE user_id = $1
                  AND start_datetime >= $2
                  AND start_datetime < $3
                ORDER BY start_datetime
                """,
                user_id,
                start_utc,
                end_utc,
            )

            if not rows:
                return "（昨日の実績なし）"

            entry_items = []
            total_minutes = 0.0
            for row in rows:
                start = row["start_datetime"].astimezone(_DEFAULT_TZ).strftime("%H:%M")
                end = row["end_datetime"].astimezone(_DEFAULT_TZ).strftime("%H:%M")
                duration = (row["end_datetime"] - row["start_datetime"]).total_seconds() / 60
                total_minutes += duration
                task = row["task_name"]
                goal = f" [{row['goal_name']}]" if row["goal_name"] else ""
                entry_items.append(f"- {start}〜{end}: {task}{goal}")

            total_hours = total_minutes / 60
            entry_items.append(f"合計: {total_hours:.1f}h ({len(rows)}セッション)")

            return "\n".join(entry_items)

    @staticmethod
    async def _get_recent_learning_notes(user_id: UUID, days: int = 3) -> str:
        """Get recent learning/diary notes from the last N days."""
        cutoff = local_today() - timedelta(days=days)
        start_utc, _ = local_date_to_utc_range(cutoff)

        async with get_connection() as conn:
            rows = await conn.fetch(
                """
                SELECT n.title, n.type, n.date, n.content, n.goal_name,
                       COALESCE(
                           (SELECT array_agg(t.name)
                            FROM note_tags nt
                            JOIN tags t ON nt.tag_id = t.id
                            WHERE nt.note_id = n.id),
                           '{}'
                       ) AS tag_names
                FROM notes n
                WHERE n.user_id = $1
                  AND n.type IN ('diary', 'learning')
                  AND n.created_at >= $2
                  AND n.archived = false
                ORDER BY n.created_at DESC
                LIMIT 5
                """,
                user_id,
                start_utc,
            )

            if not rows:
                return "（直近の学習記録・日記なし）"

            note_items = []
            for row in rows:
                date_str = row["date"].strftime("%m/%d") if row["date"] else "日付なし"
                note_type = row["type"]
                title = row["title"] or "無題"
                goal = f" [{row['goal_name']}]" if row["goal_name"] else ""
                tags = list(row["tag_names"]) if row["tag_names"] else []
                tag_str = f" tags: {', '.join(tags)}" if tags else ""

                # Content excerpt (first 100 chars)
                content = row["content"] or ""
                excerpt = content[:100].replace("\n", " ")
                if len(content) > 100:
                    excerpt += "..."

                note_items.append(f"【{date_str} {note_type}】{title}{goal}{tag_str}")
                if excerpt:
                    note_items.append(f"  {excerpt}")

            return "\n".join(note_items)

    @staticmethod
    async def _get_recent_context(user_id: UUID, limit: int = 3) -> str:
        """Get summary of recent AI interactions."""
        async with get_connection() as conn:
            rows = await conn.fetch(
                """
                SELECT situation, user_input, ai_output, created_at
                FROM ai_interactions
                WHERE user_id = $1
                ORDER BY created_at DESC
                LIMIT $2
                """,
                user_id,
                limit,
            )

            if not rows:
                return "（最近の会話なし）"

            context_items = []
            for row in rows:
                created = row["created_at"]
                situation = row["situation"]
                # Truncate for brevity
                user_input = row["user_input"][:100]
                if len(row["user_input"]) > 100:
                    user_input += "..."
                ai_output = row["ai_output"][:150]
                if len(row["ai_output"]) > 150:
                    ai_output += "..."

                context_items.append(
                    f"[{created.strftime('%m/%d %H:%M')} - {situation}]\n"
                    f"ユーザー: {user_input}\n"
                    f"AI: {ai_output}"
                )

            return "\n\n".join(context_items)
