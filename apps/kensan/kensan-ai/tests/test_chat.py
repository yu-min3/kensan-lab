"""Tests for chat.py select_tools function."""

import pytest
from kensan_ai.agents.chat import (
    select_tools,
    get_deferred_write_tools,
    ALLOWED_TOOLS,
    _has_write_intent,
)


# Use ALLOWED_TOOLS as base_tools for all tests (simulates DB config)
BASE = ALLOWED_TOOLS


class TestSelectToolsReadIntent:
    """参照系キーワードではwriteツールが含まれないことを確認。"""

    def test_goal_query_no_write_tools(self):
        """「目標達成できそう？」→ read only, no create/update/delete
        (prompt_variablesなしの場合はget_goals_and_milestonesが含まれる)"""
        result = select_tools("このままで目標達成できそう？", BASE)
        assert "get_goals_and_milestones" in result
        assert "create_goal" not in result
        assert "update_goal" not in result
        assert "delete_goal" not in result
        assert "create_milestone" not in result

    def test_task_query_no_write_tools(self):
        """「タスクの状況は？」→ get_tasks のみ (core)"""
        result = select_tools("タスクの状況は？", BASE)
        assert "get_tasks" in result
        assert "create_task" not in result
        assert "update_task" not in result
        assert "delete_task" not in result

    def test_schedule_query_no_write_tools(self):
        """「今日の予定は？」→ read only"""
        result = select_tools("今日の予定は？", BASE)
        assert "get_time_blocks" in result
        assert "create_time_block" not in result

    def test_analytics_query(self):
        """「進捗を教えて」→ analytics + review (read only)"""
        result = select_tools("進捗を教えて", BASE)
        assert "get_analytics_summary" in result or "get_daily_summary" in result
        assert "create_task" not in result

    def test_note_query_no_write(self):
        """「ノートを見せて」→ get_notes のみ"""
        result = select_tools("ノートを見せて", BASE)
        assert "get_notes" in result
        assert "create_note" not in result
        assert "update_note" not in result

    def test_no_match_defaults_read_only(self):
        """マッチなし → core + goals_read のみ (write含まない)"""
        result = select_tools("こんにちは", BASE)
        # core tools always included
        assert "get_tasks" in result
        assert "get_time_blocks" in result
        # No write tools
        assert "create_task" not in result
        assert "create_time_block" not in result


class TestSelectToolsWriteIntent:
    """書き込み意図 + ドメインキーワードがあれば write ツールも直接返す。
    ドメインキーワードなしの場合は write ツールは返さない。"""

    def test_create_task_includes_write_tools(self):
        """「タスク作って」→ write intent + "タスク" → task write 直接供給"""
        result = select_tools("新しいタスク作って", BASE)
        assert "get_tasks" in result  # core read
        assert "create_task" in result
        assert "update_task" in result
        assert "delete_task" in result

    def test_create_schedule_includes_write_tools(self):
        """「予定入れて」→ write intent + "予定" → planning write 直接供給"""
        result = select_tools("明日の午前に予定入れて", BASE)
        assert "get_time_blocks" in result
        assert "create_time_block" in result

    def test_create_goal_includes_write_tools(self):
        """「目標追加して」→ write intent + "目標" → goals_write 直接供給"""
        result = select_tools("新しい目標追加して", BASE)
        assert "get_goals_and_milestones" in result
        assert "create_goal" in result

    def test_update_task_includes_write_tools(self):
        """「タスク更新して」→ write intent + "タスク" → task write"""
        result = select_tools("このタスク更新して", BASE)
        assert "get_tasks" in result
        assert "update_task" in result

    def test_delete_task_includes_write_tools(self):
        """「タスク削除して」→ write intent + "タスク" → task write"""
        result = select_tools("このタスク削除して", BASE)
        assert "get_tasks" in result
        assert "delete_task" in result

    def test_write_note_includes_write_tools(self):
        """「メモ書いて」→ write intent + "メモ" → notes_write"""
        result = select_tools("今日のメモ書いて", BASE)
        assert "create_memo" in result
        assert "create_note" in result

    def test_generic_write_no_domain_returns_read_only(self):
        """「作って」だけ（ドメインなし）→ write なし（ドメインが不明なため）"""
        result = select_tools("これ作って", BASE)
        assert "create_time_block" not in result
        assert "create_task" not in result

    def test_schedule_plan_includes_write_tools(self):
        """「予定立てて」→ write intent + "予定" → planning write"""
        result = select_tools("明日の予定立てて", BASE)
        assert "create_time_block" in result


class TestSelectToolsSituation:
    """situation指定時のテスト。"""

    def test_weekly_situation(self):
        """weekly → review, search, read系のみ (SITUATION_TOOL_GROUPS で静的定義)"""
        result = select_tools("振り返りをお願い", BASE, situation="weekly")
        assert "get_reviews" in result
        assert "create_task" not in result

    def test_review_situation(self):
        """review → SITUATION_TOOL_GROUPS で静的定義"""
        result = select_tools("振り返りをお願い", BASE, situation="review")
        assert "get_reviews" in result
        assert "semantic_search" in result
        assert "create_task" not in result

    def test_daily_advice_situation(self):
        """daily_advice → SITUATION_TOOL_GROUPS で静的定義（planning + task を含む）"""
        result = select_tools("今日の計画", BASE, situation="daily_advice")
        assert "create_time_block" in result
        assert "create_task" in result
        assert "get_analytics_summary" in result

    def test_unknown_situation_falls_through(self):
        """未知の situation はキーワードベースのauto選択（read only）"""
        result = select_tools("今日の計画", BASE, situation="briefing")
        # "計画" は _SURU_VERB_STEMS にあるが select_tools は write を返さない
        assert "create_time_block" not in result
        assert "create_task" not in result


class TestSelectToolsContextExclusion:
    """context_keysによるツール除外テスト。"""

    def test_exclude_analytics_when_provided(self):
        """週間サマリーが渡されていればanalyticsツールを除外"""
        result = select_tools(
            "進捗を教えて", BASE, context_keys=["週間サマリー"]
        )
        assert "get_analytics_summary" not in result
        assert "get_daily_summary" not in result


class TestSelectToolsPromptVariableExclusion:
    """prompt_variablesによるツール除外テスト。"""

    def test_exclude_tasks_when_pending_tasks_injected(self):
        """pending_tasks変数があればget_tasksを除外"""
        result = select_tools(
            "タスクの状況は？", BASE, prompt_variables=["pending_tasks"]
        )
        assert "get_tasks" not in result

    def test_exclude_goals_when_goal_progress_injected(self):
        """goal_progress変数があればget_goals_and_milestonesを除外"""
        result = select_tools(
            "目標達成できそう？", BASE, prompt_variables=["goal_progress"]
        )
        assert "get_goals_and_milestones" not in result

    def test_exclude_time_blocks_when_today_schedule_injected(self):
        """today_schedule変数があればget_time_blocksを除外"""
        result = select_tools(
            "今日の予定は？", BASE, prompt_variables=["today_schedule"]
        )
        assert "get_time_blocks" not in result

    def test_exclude_time_entries_when_today_entries_injected(self):
        """today_entries変数があればget_time_entriesを除外"""
        result = select_tools(
            "今日の実績は？", BASE, prompt_variables=["today_entries"]
        )
        assert "get_time_entries" not in result

    def test_multiple_variable_exclusions(self):
        """複数変数の同時除外"""
        result = select_tools(
            "目標達成できそう？", BASE,
            prompt_variables=["pending_tasks", "goal_progress"],
        )
        assert "get_tasks" not in result
        assert "get_goals_and_milestones" not in result

    def test_chat_prompt_excludes_goals_and_tasks(self):
        """chatプロンプトの変数構成で目標達成クエリ → ツール不要"""
        # chat prompt has: current_datetime, user_memory, goal_progress, pending_tasks, recent_context
        chat_vars = ["current_datetime", "user_memory", "goal_progress", "pending_tasks", "recent_context"]
        result = select_tools(
            "このままで目標達成できそう？", BASE,
            prompt_variables=chat_vars,
        )
        assert "get_goals_and_milestones" not in result
        assert "get_tasks" not in result
        # core tools minus excluded ones
        assert "get_time_blocks" in result  # still available for schedule queries

    def test_write_tools_available_even_with_read_excluded(self):
        """変数注入でread除外されても、write intent で write ツール直接取得"""
        result = select_tools(
            "タスク作って", BASE,
            prompt_variables=["pending_tasks"],
        )
        assert "get_tasks" not in result  # read excluded by variable

        # write intent + "タスク" → task write tools directly included
        assert "create_task" in result
        assert "update_task" in result


class TestToolCount:
    """ツール数が妥当な範囲に収まることを確認。"""

    def test_read_query_tool_count(self):
        """参照系クエリではツール数が少ない"""
        result = select_tools("このままで目標達成できそう？", BASE)
        assert len(result) <= 8, f"Too many tools for read query: {len(result)} tools: {result}"

    def test_read_query_with_variables_tool_count(self):
        """変数注入時はさらにツール数が減る"""
        chat_vars = ["current_datetime", "user_memory", "goal_progress", "pending_tasks", "recent_context"]
        result = select_tools(
            "このままで目標達成できそう？", BASE,
            prompt_variables=chat_vars,
        )
        assert len(result) <= 5, f"Too many tools with variables: {len(result)} tools: {result}"

    def test_write_query_tool_count(self):
        """書き込みキーワードがあれば対応する write ツールも含む"""
        result = select_tools("タスク作って", BASE)
        assert len(result) <= 12, f"Too many tools: {len(result)} tools: {result}"
        # task write tools included (write intent + "タスク" domain)
        assert "create_task" in result
        # unrelated write tools not included
        assert "create_time_block" not in result
        assert "create_goal" not in result

    def test_no_match_tool_count(self):
        """マッチなしでもツールは最小限"""
        result = select_tools("こんにちは", BASE)
        assert len(result) <= 6, f"Too many tools for greeting: {len(result)} tools: {result}"


class TestSelectToolsSearch:
    """検索グループのツール選択テスト。"""

    def test_search_keyword_includes_search_group(self):
        """「検索して」→ search グループ含む"""
        result = select_tools("ノートを検索して", BASE)
        assert "semantic_search" in result
        assert "keyword_search" in result
        assert "hybrid_search" in result

    def test_reindex_not_in_allowed_tools(self):
        """reindex_notesはALLOWED_TOOLSに含まれない → select_toolsから除外"""
        from kensan_ai.agents.chat import TOOL_GROUPS
        # reindex_notes is in search group
        assert "reindex_notes" in TOOL_GROUPS["search"]
        # but not in ALLOWED_TOOLS
        assert "reindex_notes" not in ALLOWED_TOOLS
        # so it won't appear in result
        result = select_tools("検索して", BASE)
        assert "reindex_notes" not in result

    def test_explore_keyword_includes_search(self):
        """「探して」→ search グループ含む"""
        result = select_tools("資格について探して", BASE)
        assert "semantic_search" in result

    def test_investigate_keyword_includes_search(self):
        """「調べて」→ search グループ含む"""
        result = select_tools("サービスメッシュについて調べて", BASE)
        assert "semantic_search" in result


class TestGetDeferredWriteTools:
    """get_deferred_write_tools のテスト: read → write マッピングの検証。"""

    def test_task_read_unlocks_task_write(self):
        """get_tasks が選択されていれば task write が deferred で返る"""
        read_tools = ["get_tasks", "get_time_blocks"]
        deferred = get_deferred_write_tools(read_tools, BASE)
        assert "create_task" in deferred
        assert "update_task" in deferred
        assert "delete_task" in deferred

    def test_time_blocks_read_unlocks_planning_write(self):
        """get_time_blocks → planning write"""
        read_tools = ["get_time_blocks"]
        deferred = get_deferred_write_tools(read_tools, BASE)
        assert "create_time_block" in deferred
        assert "update_time_block" in deferred
        assert "delete_time_block" in deferred

    def test_goals_read_unlocks_goals_write(self):
        """get_goals_and_milestones → goals_write"""
        read_tools = ["get_goals_and_milestones"]
        deferred = get_deferred_write_tools(read_tools, BASE)
        assert "create_goal" in deferred
        assert "update_goal" in deferred

    def test_notes_read_unlocks_notes_write(self):
        """get_notes → notes_write"""
        read_tools = ["get_notes"]
        deferred = get_deferred_write_tools(read_tools, BASE)
        assert "create_note" in deferred
        assert "update_note" in deferred

    def test_memos_read_unlocks_notes_write(self):
        """get_memos → notes_write"""
        read_tools = ["get_memos"]
        deferred = get_deferred_write_tools(read_tools, BASE)
        assert "create_memo" in deferred

    def test_reviews_read_unlocks_review_write(self):
        """get_reviews → review (generate_review)"""
        read_tools = ["get_reviews"]
        deferred = get_deferred_write_tools(read_tools, BASE)
        assert "generate_review" in deferred

    def test_memory_read_unlocks_memory_write(self):
        """get_user_facts → memory (add_user_fact)"""
        read_tools = ["get_user_facts"]
        deferred = get_deferred_write_tools(read_tools, BASE)
        assert "add_user_fact" in deferred

    def test_no_read_tools_returns_empty(self):
        """read ツールなし → 空"""
        deferred = get_deferred_write_tools([], BASE)
        assert deferred == []

    def test_unrelated_read_tools_returns_empty(self):
        """マッピングにない read ツール → 空"""
        deferred = get_deferred_write_tools(["get_analytics_summary"], BASE)
        assert deferred == []

    def test_respects_base_tools(self):
        """base_tools にないツールは deferred に含まれない"""
        limited_base = ["get_tasks", "create_task"]  # update/delete は不許可
        deferred = get_deferred_write_tools(["get_tasks"], limited_base)
        assert "create_task" in deferred
        assert "update_task" not in deferred
        assert "delete_task" not in deferred

    def test_only_write_tools_returned(self):
        """deferred にはwrite ツールのみ（read ツールは含まない）"""
        # review グループには get_reviews, get_review, generate_review がある
        # generate_review のみ write
        deferred = get_deferred_write_tools(["get_reviews"], BASE)
        assert "get_reviews" not in deferred
        assert "get_review" not in deferred

    def test_end_to_end_schedule_flow(self):
        """「予定よろしく」のE2Eフロー: select_tools → get_deferred_write_tools"""
        # select_tools で read ツールを取得
        selected = select_tools("予定よろしく", BASE)
        assert "get_time_blocks" in selected
        assert "create_time_block" not in selected

        # deferred で write ツールを取得
        deferred = get_deferred_write_tools(selected, BASE)
        assert "create_time_block" in deferred
        assert "update_time_block" in deferred

    def test_end_to_end_greeting_flow(self):
        """「こんにちは」のE2Eフロー: deferred も最小限"""
        selected = select_tools("こんにちは", BASE)
        deferred = get_deferred_write_tools(selected, BASE)
        # core (get_tasks, get_time_blocks, etc.) + goals_read → 対応する write
        assert "create_task" in deferred
        assert "create_time_block" in deferred

    def test_end_to_end_write_intent_flow(self):
        """「タスク作って」のE2Eフロー: write ツール直接供給 + deferred に重複なし"""
        selected = select_tools("タスク作って", BASE)
        assert "get_tasks" in selected
        # write intent + "タスク" → task write 直接供給
        assert "create_task" in selected
        assert "update_task" in selected
        assert "delete_task" in selected

        # deferred は直接供給されたツールを含まない
        deferred = get_deferred_write_tools(selected, BASE)
        assert "create_task" not in deferred
        assert "update_task" not in deferred
        assert "delete_task" not in deferred
        # planning write はまだ deferred（get_time_blocks が core にあるため）
        assert "create_time_block" in deferred

    def test_deferred_excludes_already_selected(self):
        """直接供給された write ツールは deferred に含まれない"""
        # Simulate: select_tools returned task write tools directly
        selected = ["get_tasks", "get_time_blocks", "create_task", "update_task", "delete_task"]
        deferred = get_deferred_write_tools(selected, BASE)
        # task write already in selected → excluded from deferred
        assert "create_task" not in deferred
        assert "update_task" not in deferred
        assert "delete_task" not in deferred
        # planning write still deferred
        assert "create_time_block" in deferred


class TestWriteIntentStemMatching:
    """語幹ベースの書き込み意図判定テスト。活用形のバリエーションをカバーできることを確認。
    _has_write_intent はユーティリティとして残されており、直接テスト可能。"""

    # サ変動詞: 名詞部分で全活用形にマッチ
    @pytest.mark.parametrize("msg", [
        "予定を作成して",     # て形
        "予定を作成したい",    # たい形
        "予定を作成しよう",    # 意志形
        "予定を作成する",     # 辞書形
        "予定を作成した",     # た形
    ])
    def test_suru_verb_conjugations(self, msg):
        """サ変動詞「作成する」の各活用形でwrite intent検出"""
        assert _has_write_intent(msg) is True

    # 五段動詞: 語幹＋活用行で全活用形にマッチ
    @pytest.mark.parametrize("msg", [
        "タスク作って",     # て形
        "タスク作りたい",    # たい形
        "タスク作ろう",     # 意志形
        "タスク作る",      # 辞書形
        "タスク作れる",     # 可能形
        "タスク作った",     # た形（促音便）
    ])
    def test_godan_verb_conjugations(self, msg):
        """五段動詞「作る」の各活用形でwrite intent検出"""
        assert _has_write_intent(msg) is True

    # 一段動詞: 語幹で全活用形にマッチ
    @pytest.mark.parametrize("msg", [
        "予定に入れて",     # て形
        "予定に入れたい",    # たい形
        "予定に入れよう",    # 意志形
        "予定に入れる",     # 辞書形
    ])
    def test_ichidan_verb_conjugations(self, msg):
        """一段動詞「入れる」の各活用形でwrite intent検出"""
        assert _has_write_intent(msg) is True

    # 参照系はマッチしないことを確認
    @pytest.mark.parametrize("msg", [
        "今日の予定は？",
        "タスクの状況は？",
        "こんにちは",
        "目標の進捗を教えて",
    ])
    def test_read_intent_not_matched(self, msg):
        """参照系メッセージではwrite intentが検出されない"""
        assert _has_write_intent(msg) is False


class TestSelectToolsNoFiles:
    """ファイル関連ツールが存在しないことのテスト。"""

    def test_no_upload_file_in_allowed_tools(self):
        """upload_fileがALLOWED_TOOLSに含まれない"""
        assert "upload_file" not in ALLOWED_TOOLS

    def test_no_download_file_in_allowed_tools(self):
        """download_fileがALLOWED_TOOLSに含まれない"""
        assert "download_file" not in ALLOWED_TOOLS

    def test_no_files_tool_group(self):
        """filesツールグループが存在しない"""
        from kensan_ai.agents.chat import TOOL_GROUPS
        assert "files" not in TOOL_GROUPS
