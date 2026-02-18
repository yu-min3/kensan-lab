-- 004: Fix schedule proposal date range instructions in chat context
-- Problem: AI was proposing schedules for next week when user asked for "今週"
-- because the prompt lacked explicit date range guidance.

UPDATE ai_contexts
SET system_prompt = REPLACE(
  REPLACE(
    system_prompt,
    '- 「今週の予定を立てて」→ {weekly_summary}で今週の時間配分を把握済み。get_time_blocks + get_tasks で既存の予定と未消化タスクを取得 → 生産性ピーク時間帯({user_patterns})を活かして create_time_block をまとめて提案',
    '- 「今週の予定を立てて」→ {weekly_summary}で今週の時間配分を把握済み。get_time_blocks(start_date=今日, end_date=今週日曜) + get_tasks で既存の予定と未消化タスクを取得 → 生産性ピーク時間帯({user_patterns})を活かして、**今日から今週日曜日まで**の範囲で create_time_block をまとめて提案'
  ),
  '- 日付は JST 基準。「今日」「明日」等は JST で解釈する
- 曖昧な時間',
  '- 日付は JST 基準。「今日」「明日」等は JST で解釈する
- スケジュール提案の日付範囲: 「今週」→ 今日〜今週の日曜日（{current_datetime}の週）。「来週」→ 次の月曜〜日曜。時期指定なし→ 今日〜今週日曜。**ユーザーが指定した期間を超えて提案しない**（「今週」と言われたら来週の日付を含めない）
- 曖昧な時間'
)
WHERE situation = 'chat';
